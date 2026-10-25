# Plan: Leases, Failure Intelligence, Idempotency

**Scope:** three showcase features on the existing jobqueue base. No real SMTP/PDF/LLM product work beyond a **Grok** call for *new* failure fingerprints. Processors stay fixtures (`echo`, `fail_until_n`, `sleep` — keep current mocks if you prefer).

**Out of scope:** fair multi-tenant dequeue, job DAGs, UI, Kubernetes.

---

## 1. What we are building (one picture)

```text
Client
  Idempotency-Key ──► POST /job/submit
                         │
                         ├─ unique (project_id, key) → same job_id if replay
                         ├─ INSERT jobs + job_events (queued)
                         └─ XADD stream:high | stream:default  {job_id}

Workers (consumer group)
  XREADGROUP BLOCK
  claim = lease (message in PEL until XACK)
  heartbeat / idle → XCLAIM by another worker
  process → XACK
  on fail: retry (XADD again or XPADD) OR permanent fail

Permanent fail
  jobs.status = failed
  INSERT failure_events + fingerprint
  if fingerprint NEW → Grok once → store diagnosis
  job_events append failed
  DLQ is Postgres-first (not a hidden Redis list)

Client / operator
  GET  /jobs/{id}           + events timeline
  GET  /failures            grouped by fingerprint
  GET  /failures/{fp}       list jobs + Grok summary
  POST /failures/{jobID}/requeue
```

**Thesis you defend:** workers are **at-least-once with leases**; the HTTP API is **idempotent**; failures are **incidents**, not a Redis dump.

---

## 2. Feature 2 — Leases via Redis Streams

### 2.1 Why lists are the wrong end-state

Today: `LPUSH` / `BRPOP` job UUID. If a worker dies after `BRPOP` and before `XACK`-equivalent, the ID is **gone from Redis**. Recovery is the reaper (1 hour, status=`running`). That is a **lost-work window**.

**Streams + consumer groups** keep unacked messages in the **Pending Entries List (PEL)**. Another worker can `XCLAIM` them after idle time. That *is* the lease.

### 2.2 Redis concepts to learn (prepare this)

| Topic | Why you need it |
|-------|-----------------|
| **List vs Stream** | List: pop removes. Stream: append-only log + IDs (`ms-seq`). |
| **`XADD` key * field value** | Enqueue. Fields: `job_id` (and maybe `attempt`). |
| **Consumer group** | `XGROUP CREATE stream:high workers $ MKSTREAM` — group name `workers`, consumers = worker IDs. |
| **`XREADGROUP GROUP workers c1 BLOCK 5000 COUNT 1 STREAMS stream:high >`** | `>` = never delivered to this group. New work. |
| **PEL** | After read, message is pending for that consumer until `XACK`. |
| **`XACK`** | Lease complete — drop from PEL. Call **after** durable Postgres status change you consider “done for this delivery” (completed, or failed+DLQ written, or retry re-enqueued). |
| **`XPENDING` + `XCLAIM`** | Idle pending > `lease_ms` → claim for this consumer. Replaces the 1-hour reaper for **in-flight Redis delivery**. |
| **`XAUTOCLAIM`** (Redis 6.2+) | Simpler reclaim loop. Prefer this if Redis version allows. |
| **At-least-once** | Crash after process but before ACK → **re-delivery**. Worker **must** stay idempotent (`FOR UPDATE` + skip if not `queued` / already terminal). |
| **Never ACK then lose DB** | Order: commit job state → then ACK. If ACK then crash before DB: job can look queued in DB but gone from stream — **bad**. If DB then crash before ACK: re-delivery + idempotent skip — **OK**. |

**Also learn:** consumer names must be **stable per process** (e.g. `hostname-pid-workerN`) so PEL is attributable; on shutdown, don’t leave PEL forever — either ACK, or let claimer take over.

### 2.3 Mapping onto this repo

| Today | After |
|-------|--------|
| `heuristics.GetQueue` → `queue:high` / `queue:default` | Same logical names, Redis type = **stream** keys `stream:high`, `stream:default` (or keep names, change commands only). |
| `LPush` in submit, retry, reaper | `XADD` |
| `BRPop` in worker | `XREADGROUP` |
| Reaper 1h `running` | **Claimer** every few seconds: `XAUTOCLAIM` idle > `LEASE` (e.g. 30–60s). Optional **safety net**: still reset DB `running` older than `2 * LEASE` if Redis and DB diverge. |
| Autoscaler `LLEN` | `XLEN` or `XPENDING` count (pending + lag). Lag = new messages not yet read. |

**Lease duration:** start **30s**, heartbeat by processing quickly; long jobs (`sleep`) must **extend** (periodic `XCLAIM` of own pending with idle 0, or split work). For showcase, cap fixture jobs to < lease or implement a simple heartbeat goroutine per in-flight message.

**Pool:** each worker loop is one **consumer** in group `g:high` / `g:default`. Scale up = more consumers on same group (correct). Scale down = stop reading; PEL of dead consumer must be claimed (claimer uses `XAUTOCLAIM` **across** the group, not only own consumer).

### 2.4 Implementation slices (feature 2)

1. Config: `REDIS_STREAM_HIGH`, lease TTL, group name.
2. Startup: `XGROUP CREATE ... MKSTREAM` (ignore BUSYGROUP).
3. Enqueue helper: `XADD` job_id only (Postgres remains source of truth).
4. Worker: `XREADGROUP` → process (existing lock logic) → `XACK` on success path and on permanent-fail path; on retry: update DB queued, `XADD` new entry (or same stream), `XACK` old delivery so PEL doesn’t retry the *same* message forever **and** double-run — **decision:** retry = ACK old + XADD new (cleaner) vs leave pending (wrong).
5. Claimer goroutine (replaces/shrinks reaper): idle pending → `XCLAIM` → process or re-queue status.
6. Autoscaler: use stream length / pending.
7. Remove list-based enqueue from submit/retry.

**Retry rule (write this in code comments):** each **delivery** is one stream message. Retry = new `XADD` + ACK previous. Prevents infinite PEL for the same ID.

---

## 3. Feature 5 — Idempotency + job_events

### 3.1 Concepts to learn

| Topic | Why |
|-------|-----|
| **HTTP Idempotency-Key** (Stripe-style) | Client retries POST after timeout without creating two jobs. |
| **Fingerprint of request** | Store hash of body; same key + **different** body → **409 Conflict**. |
| **Unique constraint** | `UNIQUE(project_id, idempotency_key)` — race: two concurrent POSTs, one wins, other reads winner. |
| **Event sourcing lite** | Append-only `job_events`; job row is **projection** (current status). |
| **At-least-once vs exactly-once** | Exactly-once **end-to-end** is a lie; you offer **exactly-once submit** + **at-least-once execute** + idempotent handlers. |

### 3.2 Schema

**`jobs` additions**

- `idempotency_key` nullable string  
- `request_hash` string (sha256 of canonical JSON body)  
- Unique index: `(project_id, idempotency_key)` WHERE key IS NOT NULL  

**`job_events`**

| Column | Meaning |
|--------|---------|
| `id` | uuid |
| `job_id` | fk |
| `type` | `queued`, `running`, `retry`, `completed`, `failed`, `requeued`, `claimed` |
| `payload` | jsonb optional (error message, stream_id, worker_id) |
| `created_at` | timestamptz |

Append on every transition in **the same transaction** as the job row update when possible.

### 3.3 Submit user flow

1. Client sends `Idempotency-Key: <uuid>` (required for showcase? **Recommend required on submit** so the feature is always exercised; or required if header present — prefer **required** for the public story).
2. Server hashes body (`project_id`, `type`, `payload` only — ignore extra fields).
3. If row exists for `(project, key)`:
   - same hash → **200/202** same `job_id` (not a second enqueue)
   - different hash → **409**
4. If new: insert job + event `queued` + `XADD` in this order: **DB commit first**, then stream. If stream fails: job stays `queued` without stream — **claimer/sweeper** (or submit retry) `XADD` if queued and not in stream. Document this gap; a small “enqueue queued jobs missing from stream” loop is the fix.

**Response:** include `job_id`, `idempotent_replay: true|false`.

### 3.4 Status user flow

`GET /job/status/{id}` returns job **plus** `events[]` chronological. This is the timeline you demo.

---

## 4. Feature 1 — Failure intelligence + complete DLQ

### 4.1 What “DLQ complete” means here

DLQ is **not** a Redis list operators cannot see. It is:

1. `jobs.status = failed` (already)
2. **`failure_events`** row (durable)
3. **Fingerprint + incident grouping**
4. **HTTP API** to list / inspect / requeue
5. **Grok** only when a fingerprint is seen the **first** time (or first time per project — pick one: **global fingerprint** is simpler for demos; **per-project** is more multi-tenant)

Redis `queue:dlq` JSON list can be **dropped** or kept as a mirror; **Postgres is source of truth**.

### 4.2 Fingerprint (no LLM required)

```text
normalize(error) = lowercase, strip digits/UUIDs/timestamps
fingerprint = sha256(job.Type + "|" + error_class + "|" + sorted payload keys)
```

- **error_class:** first line of error, or `unknown` if empty  
- **payload keys only** (not values) so PII doesn’t explode cardinality  
- Store `error_raw` on the event for humans  

Same fingerprint → increment `failure_incidents.occurrence_count`, attach `job_id`.

### 4.3 Grok (xAI) — when and how

**When:** insert incident if fingerprint unseen → call Grok **once** → save `diagnosis` on `failure_incidents`. Later jobs with same fingerprint **skip** Grok.

**Config:** `GROK_API_KEY`, `GROK_MODEL` (e.g. `grok-4` / current chat model), `GROK_BASE_URL=https://api.x.ai/v1`

**Call:** OpenAI-compatible `POST /v1/chat/completions` — system prompt: “You diagnose background job failures. Be short. Suggest retry vs poison vs payload bug.” User: type, error, payload **keys**, retry count, duration.

**Failure of Grok:** incident still exists; `diagnosis` null; retry diagnosis in background (don’t block worker). Worker must not wait on HTTP to Grok — `go` or queue `diagnosis_pending`.

**Cost/stress:** Grok is **not** on the hot path; stress tests should **stub** Grok or use a fake client.

### 4.4 Schema

**`failure_incidents`**

- `fingerprint` PK (or uuid + unique fingerprint)
- `job_type`, `error_class`, `payload_keys` jsonb
- `diagnosis` text nullable (Grok)
- `occurrence_count` int
- `first_seen_at`, `last_seen_at`
- optional `project_id` if grouping per project

**`failure_events`**

- `job_id`, `incident_id`/`fingerprint`, `error_raw`, `retry_count`, `duration_ms`, `created_at`

### 4.5 DLQ HTTP (protected, project-scoped)

| Method | Path | Behavior |
|--------|------|----------|
| `GET` | `/api/v1/failures` | Incidents for caller’s projects (`?projectID=`). Sort by `last_seen_at`. |
| `GET` | `/api/v1/failures/{fingerprint}` | Incident + Grok diagnosis + recent job ids |
| `GET` | `/api/v1/failures/jobs/{jobID}` | One failed job + events + incident |
| `POST` | `/api/v1/failures/jobs/{jobID}/requeue` | `status=queued`, `retry_count=0` (or keep count — **document**), event `requeued`, `XADD` |
| `DELETE` | `/api/v1/failures/jobs/{jobID}` | Optional: mark `discarded` so it leaves “open DLQ” views |

Auth: same `loadOwnedProject` as jobs.

### 4.6 User flows

**A. App developer (happy + retry)**  
Register → project → submit with Idempotency-Key → poll status → see `events`.

**B. App developer (crash)**  
Submit `fail_until_n` or mock email that fails → retries appear as `retry` events → still `queued`/`running`.

**C. Operator (poison)**  
Job hits max retries → `failed` → `GET /failures` shows **one row** if 50 jobs same fingerprint → open fingerprint → read Grok text → `POST requeue` after “fixing” payload (optional body: new payload JSON).

**D. Client timeout**  
POST twice same key → one job, one stream message.

---

## 5. Processors (fixtures only)

Keep or add:

| Type | Role |
|------|------|
| `echo` / `summarize_text` | always success |
| `fail_until_n` | payload `{ "fail_times": 2 }` — deterministic retries |
| `always_fail` | poison → DLQ + fingerprint |
| `sleep` | payload `{ "ms": 5000 }` — lease/heartbeat demo |
| existing mocks | optional; email random fail is **bad for tests** — replace tests with `fail_until_n` |

Register in `main.go`; add to `AllowedJobTypes`.

---

## 6. Implementation order (do in this sequence)

Build **bottom-up** so each slice is demoable.

| Step | Work | Demo |
|------|------|------|
| **A** | `job_events` + write on existing transitions (still lists OK) | Status returns timeline |
| **B** | Idempotency-Key + unique index + replay/409 | Double POST |
| **C** | Streams enqueue + XREADGROUP + ACK; dual-write period optional | Jobs still complete |
| **D** | Remove lists; claimer (`XAUTOCLAIM`); shrink reaper | Kill `-9` worker, job resumes |
| **E** | `failure_events` + incidents + fingerprint on permanent fail | `GET /failures` |
| **F** | DLQ requeue API | Failed → queued again |
| **G** | Grok on new fingerprint (async) | Diagnosis text on incident |
| **H** | Chaos + stress tests | CI / local script |

Do **not** start with Grok. Do **not** start with Streams and events in the same PR if you can avoid it — A→B first (pure Postgres), then C→D (Redis), then E→G (DLQ product).

---

## 7. Tests

### 7.1 Feature tests (must prove 1, 2, 5)

**Idempotency**

- Same key + same body → one job row, one stream message (or `XLEN` +1 once).
- Same key + different body → 409, original job unchanged.
- Two goroutines same key → one winner, both return same `job_id` (transaction / unique violation retry).
- Missing key → 400 if you made it required.

**Events**

- Submit → at least `queued`.
- Success → `queued`, `running`, `completed` in order.
- `fail_until_n` → `retry` then `completed`.
- `always_fail` → `failed` + failure_event.
- Requeue → `requeued` then later `running`.

**Streams / leases**

- Happy path: message ACKed, not in PEL.
- Inject: process succeeds, skip ACK, run claimer → second delivery, idempotent (no double `completed` side effect — fixture should be safe).
- Kill worker mid-job (`sleep`): after idle, another consumer claims; job completes once.
- Retry: PEL empty for old id; new `XADD` exists.

**Failures / DLQ**

- N `always_fail` with same type + same payload keys + same error → **one** incident, `occurrence_count=N`.
- Different payload keys → different fingerprint.
- Requeue owned job → 202 and job queued; other user’s job → 403.
- Grok: first fingerprint calls client **once**; second job same fp **zero** calls (mock clock / mock HTTP).

### 7.2 Regression (everything else still works)

Keep Phase 1 behavior:

- Register / login / projects CRUD-list
- Submit validation (`unknown type` 400)
- Submit without owning project → 403
- Status/list scoping
- Rate limit still applied
- Metrics still increment (`jobs_processed_total`, failures)
- Autoscaler still reads depth (after switching to `XLEN`)
- Graceful shutdown: in-flight finishes or is claimed, not silent drop
- `GET /metrics` 200

**API-level integration tests** (httptest + testcontainers **or** docker Redis/Postgres in CI): one `TestMain` stack, run the table above. Today `test/api_test.go` is empty — this is the right time to fill it.

### 7.3 Chaos / stress (you said this is OK)

| Test | Pass criteria |
|------|----------------|
| N=100 concurrent submits, unique keys | 100 jobs, 100 stream entries |
| N=100 concurrent same idempotency key | 1 job |
| Burst 1k `echo` jobs | all `completed`; no PEL leak after drain |
| Random SIGKILL of 50% workers during `sleep` jobs | all jobs `completed` or `failed` within bound; **zero** stuck `running` past `2*lease` |
| Poison flood `always_fail` | one (or few) incidents, Grok mock called ≪ job count |

Grok **mocked** in stress. Optional live Grok test behind `GROK_LIVE=1`.

---

## 8. Config / ops

```env
POSTGRES_DSN=...
REDIS_URL=...
PORT=8080
GROK_API_KEY=          # optional; skip diagnosis if empty
GROK_MODEL=grok-4
GROK_BASE_URL=https://api.x.ai/v1
JOB_LEASE_MS=30000
STREAM_GROUP=workers
```

Startup: AutoMigrate new tables; `XGROUP CREATE`.

---

## 9. Files you will touch (expected)

| Area | Files |
|------|--------|
| Models | `models.go` — Job fields, JobEvent, FailureIncident, FailureEvent |
| Enqueue | `jobs/enqueue.go` — XADD |
| Worker | `worker.go` — XREADGROUP, ACK; `claimer.go` (new); slim `reaper.go` |
| Pool | consumer name per worker |
| API | submit idempotency; status+events; `failures.go` routes |
| AI | replace stub with `ai/grok.go` + interface `Diagnoser` |
| Tasks | `fail_until_n`, `always_fail`, `sleep` |
| Tests | `internal/jobs`, `internal/workers`, `test/api_test.go`, chaos under `test/chaos_test.go` |
| Config | `constants.go` stream names |

---

## 10. What you should be able to explain in an interview

1. **Why Streams:** PEL = lease; lists delete on pop.  
2. **ACK order:** DB then ACK.  
3. **Idempotent worker:** re-delivery after crash.  
4. **Idempotent client:** unique key ≠ exactly-once processing.  
5. **Fingerprint vs LLM:** grouping is deterministic; Grok is commentary on **new** groups.  
6. **DLQ:** failed jobs are queryable incidents + requeue, not `LRANGE queue:dlq`.

---

## 11. Success criteria (done)

- [ ] Kill a worker; job completes via claim, not 1-hour reaper  
- [ ] Double POST same key → one job  
- [ ] Status shows full event timeline  
- [ ] 20 identical poisons → 1 incident, Grok once (or skip if no key)  
- [ ] Requeue from API works  
- [ ] Chaos script green against local Redis/Postgres  

When those hold, the repo is no longer “generic BRPOP tutorial”; it is a **leased, auditable, incident-oriented job system** with an optional Grok note on new failure classes.
