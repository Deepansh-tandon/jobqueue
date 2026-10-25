# Jobqueue — Complete Codebase Explained

Every file, every function, every non-trivial line explained from the ground up.  
No Go knowledge assumed. By the end you will understand the language patterns, the libraries, and why every decision was made.

---

## Table of Contents

1. [Go Language Primer (concepts used here)](#1-go-language-primer)
2. [Project Layout](#2-project-layout)
3. [go.mod — Dependency manifest](#3-gomod)
4. [config/constants.go — Central definitions](#4-configconstantsgo)
5. [config/env.go — Loading settings](#5-configenvgo)
6. [models/models.go — Database shapes](#6-modelsmodelsgo)
7. [internal/api/api.go — API struct](#7-internalapiapigo)
8. [internal/api/routes.go — URL routing](#8-internalapiroutesgo)
9. [internal/api/user.go — Register & Login](#9-internalapiusergo)
10. [internal/api/project.go — Project handlers](#10-internalapiprojectgo)
11. [internal/api/job.go — Job handlers](#11-internalapijobgo)
12. [internal/middleware/auth.go — API key authentication](#12-internalmiddlewareauthgo)
13. [internal/middleware/ratelimit.go — Rate limiting](#13-internalmiddlewareratelimitgo)
14. [internal/jobs/validator.go — Input validation](#14-internaljobsvalidatorgo)
15. [internal/jobs/enqueue.go — Redis enqueue helper](#15-internaljobsenqueuego)
16. [internal/heuristics/router.go — Queue routing logic](#16-internalheuristicsroutergo)
17. [internal/tasks/processor.go — Task registry & mocks](#17-internaltasksprocessorgo)
18. [internal/tasks/receipt.go — PDF receipt generator](#18-internaltasksreceiptgo)
19. [internal/workers/pool.go — Worker pool](#19-internalworkerspoolgo)
20. [internal/workers/worker.go — Job execution](#20-internalworkersworkergo)
21. [internal/workers/autoscaler.go — Autoscaling](#21-internalworkersautoscalergo)
22. [internal/workers/reaper.go — Stuck job recovery](#22-internalworkersreapergo)
23. [internal/ai/client.go — AI struct](#23-internalaiclientgo)
24. [internal/ai/summarizer.go — DLQ AI summary](#24-internalaisummarizergo)
25. [internal/monitoring/prometheus.go — Metrics](#25-internalmonitoringprometheusgo)
26. [cmd/server/main.go — Entry point (wire everything)](#26-cmdservermaingothe-entry-point)
27. [Test files explained](#27-test-files)
28. [Data & control flow end-to-end](#28-end-to-end-flow)

---

## 1. Go Language Primer

Before reading any code you need these six Go ideas. Everything in the project is built from them.

### 1.1 Packages

Every `.go` file starts with `package name`. A package is a namespace — files in the same directory share the same package name and can see each other's unexported symbols. Two directories = two packages even if they have the same name.

```go
package main   // special: a package named main with a main() function is the runnable program
package api    // library package — imported by other packages
```

### 1.2 Imports

```go
import (
    "fmt"                          // standard library path
    "github.com/go-chi/chi/v5"     // third-party module from GitHub
    "jobqueue/internal/models"     // our own package — path relative to module root
)
```

The module root is declared in `go.mod` as `module jobqueue`. So `jobqueue/internal/models` maps to the directory `internal/models/` inside the project.

### 1.3 Functions

```go
func Add(a int, b int) int {
    return a + b
}
```

- Multiple return values: `func Divide(a, b int) (int, error)` — idiomatic Go way to return a result AND an error.
- Named receiver (method): `func (u *User) Greet() string` — `Greet` belongs to the type `*User`.

### 1.4 Structs & pointers

```go
type Config struct {
    Port string
    Debug bool
}

cfg := Config{Port: "8080"}   // value (a copy)
cfgPtr := &cfg                // pointer to cfg (address)
cfgPtr.Port = "9090"          // mutates original
```

`*T` means "pointer to T". `&x` gives you a pointer to `x`. Most structs in this project are passed as pointers so mutations are shared, not copied.

### 1.5 Interfaces

```go
type Processor interface {
    Process(ctx context.Context, job models.Job) error
}
```

An interface is a contract: any type that has a `Process` method with that exact signature satisfies this interface — no explicit declaration needed. This is Go's polymorphism.

### 1.6 Goroutines & channels

```go
go someFunction()   // runs concurrently in a new lightweight thread
```

A channel (`chan`) sends data between goroutines. `ctx.Done()` is a special receive-only channel that closes when the context is cancelled. The `select` statement waits on multiple channels at once — like a `switch` for channels.

### 1.7 `defer`

```go
defer logger.Sync()
```

`defer` schedules a call to run when the surrounding function returns, regardless of how it returns (normal or panic). Used for cleanup: closing files, unlocking mutexes, committing/rolling back transactions.

### 1.8 Short variable declaration

```go
x := 5          // declare AND assign; type inferred as int
x, err := f()   // declare multiple; if err already exists, it is just reassigned
```

### 1.9 Error handling idiom

```go
result, err := someFunction()
if err != nil {
    // handle it — return, log, panic
}
// use result safely below
```

Go has no exceptions. Errors are plain values returned as the last return value. You check them explicitly every time.

---

## 2. Project Layout

```
jobqueue/
│
├── cmd/
│   └── server/
│       └── main.go          ← the runnable program; wires everything
│
├── internal/                ← Go rule: packages here cannot be imported from outside this module
│   ├── ai/                  ← DLQ AI summarizer
│   ├── api/                 ← HTTP handlers, router
│   ├── config/              ← env loading, constants
│   ├── heuristics/          ← maps job type → Redis queue name
│   ├── jobs/                ← validation, enqueue helper
│   ├── middleware/          ← auth, rate limit (HTTP middleware)
│   ├── models/              ← Go structs that map to Postgres tables
│   ├── monitoring/          ← Prometheus metrics
│   ├── tasks/               ← Processor interface + implementations
│   └── workers/             ← pool, worker loop, autoscaler, reaper
│
├── docs/                    ← this file, architecture.md
├── go.mod                   ← module name + dependencies
├── go.sum                   ← locked checksums for every dependency
├── Dockerfile
├── docker-compose.yml
└── .env                     ← local secrets (gitignored)
```

**Why `internal/`?**  
The Go toolchain enforces that nothing outside this module can import `jobqueue/internal/...`. This gives you a hard boundary: your internal implementation is never accidentally used by external callers.

**Why `cmd/server/`?**  
The `cmd/` convention means "runnable binaries live here". Each sub-directory is a separate binary. Right now there is only one (`server`). Later you might add `cmd/worker` or `cmd/scheduler`.

---

## 3. go.mod

**File:** `go.mod`

```
module jobqueue

go 1.23.0

toolchain go1.24.4
```

- `module jobqueue` — declares the module name. Every import path inside this project starts with `jobqueue/`.
- `go 1.23.0` — minimum Go version required.
- `toolchain go1.24.4` — the exact toolchain used to build (newer features available but 1.23 is the floor).

### Direct dependencies explained

```
github.com/go-chi/chi/v5 v5.2.2
```
`chi` — HTTP router. Registers URL patterns and calls handlers. `v5` is the major version (in Go modules, breaking-change versions get a suffix).

```
github.com/go-redis/redis/v8 v8.11.5
```
Redis client library. Wraps Redis commands (`LPUSH`, `BRPOP`, `SET`, `LLEN`, `PING`, etc.) as Go method calls.

```
github.com/google/uuid v1.4.0
```
Generates random UUIDs. Used for `User.ID`, `Project.ID`, `Job.ID`, `User.APIKey`.

```
github.com/patrickmn/go-cache v2.1.0+incompatible
```
In-memory TTL cache (like a map that expires entries). Used to store per-API-key rate limiter instances.

```
github.com/prometheus/client_golang v1.22.0
```
Prometheus metrics library. Lets the app expose counters, gauges, and histograms at `/metrics`.

```
go.uber.org/zap v1.27.0
```
Structured, levelled logger (Info, Warn, Error, Fatal, Debug). Much faster than `fmt.Println`.

```
golang.org/x/crypto v0.39.0
```
Extended crypto: specifically `bcrypt` for password hashing.

```
golang.org/x/time v0.12.0
```
Token bucket rate limiter (`rate.Limiter`).

```
gorm.io/gorm v1.30.0
gorm.io/driver/postgres v1.6.0
```
GORM is an ORM (Object-Relational Mapper). You write Go structs; GORM generates SQL. The postgres driver is the underlying database connector.

```
github.com/joho/godotenv v1.5.1
```
Loads a `.env` file into environment variables so `os.Getenv` works during local development.

```
github.com/jung-kurt/gofpdf v1.16.2
```
Pure-Go PDF generation. Used by `ReceiptGenerator`.

```
github.com/stretchr/testify v1.10.0
```
Testing helpers: `assert.Equal`, `require.NoError`, etc. Makes test assertions readable.

---

## 4. config/constants.go

**File:** `internal/config/constants.go`

```go
package config
```
Declares this file belongs to the `config` package.

```go
const (
    QueueHigh    = "queue:high"
    QueueDefault = "queue:default"
)
```
`const` declares compile-time constants — values baked in at build time, never change at runtime.  
`const ( ... )` is a block of constants (same as writing `const QueueHigh = ...` twice but grouped).

These are the **Redis list names** that worker pools listen on. Using constants instead of string literals everywhere means if you ever rename a queue, you change it in one place.

```go
const (
    JobTypeSendEmail        = "send_email"
    JobTypeGenerateReceipt  = "generate_receipt"
    JobTypeSummarizeText    = "summarize_text"
)
```
The three **canonical job type strings**. These must exactly match:
- What the client sends in `POST /api/v1/job/submit` as `"type"`
- What `tasks.Register(...)` uses in `main.go`
- What `heuristics.GetQueue` switches on

```go
var AllowedJobTypes = map[string]struct{}{
    JobTypeSendEmail:       {},
    JobTypeGenerateReceipt: {},
    JobTypeSummarizeText:   {},
}
```
`var` declares a package-level variable (mutable, but in practice treated as read-only here).

`map[string]struct{}` — a map whose keys are strings and whose values are empty structs (`struct{}`).  
Why `struct{}`? Because it takes **zero bytes**. This is Go's idiomatic "set" — you only care whether a key exists, not what value it has.

`{}` on the right of each entry initialises the empty struct value (the zero value for `struct{}` is also `{}` — this is just explicit).

To check membership:
```go
_, ok := config.AllowedJobTypes["send_email"]
// ok == true means it exists
```
The `_` discards the value (which is always `struct{}{}`). We only care about `ok`.

---

## 5. config/env.go

**File:** `internal/config/env.go`

```go
type Config struct {
    PostgresDSN string
    RedisURL    string
    Port        string
}
```
A plain struct holding the three configuration values the app needs. No tags needed here — this is not serialized to JSON or a database.

```go
func Load() (*Config, error) {
```
Returns a pointer to a `Config` and an error. The caller checks the error, and if `nil`, uses `*Config`.

```go
    _ = godotenv.Load()
```
Calls `godotenv.Load()` which reads `.env` from the current working directory and calls `os.Setenv` for each `KEY=VALUE` line.  
`_ =` discards the return value. We deliberately ignore the error: in production there is no `.env` file and that is fine — environment variables are already set.

```go
    dsn := os.Getenv("POSTGRES_DSN")
    redisURL := os.Getenv("REDIS_URL")
```
`os.Getenv` reads an environment variable. Returns `""` if not set.

```go
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
```
Default to port 8080 if `PORT` is not set. Simple fallback pattern.

```go
    return &Config{
        PostgresDSN: dsn,
        RedisURL:    redisURL,
        Port:        port,
    }, nil
```
Constructs a `Config` by name (`FieldName: value`), takes its address with `&`, and returns it alongside `nil` error (success).

---

## 6. models/models.go

**File:** `internal/models/models.go`

This file defines the **database tables** as Go structs. GORM reads these structs and creates/alters tables to match.

```go
import "time"
```
Only `time` is needed — for `time.Time` fields (timestamps in Postgres).

### Status constants

```go
const (
    StatusQueued    = "queued"
    StatusScheduled = "scheduled"
    StatusRunning   = "running"
    StatusCompleted = "completed"
    StatusFailed    = "failed"
)
```
All possible values for `Job.Status`. Defined here so they can be imported in workers and handlers without creating a circular import.

### User struct

```go
type User struct {
    ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    Email     string    `gorm:"unique;not null"`
    Password  string    `gorm:"not null"`
    APIKey    string    `gorm:"unique;not null"`
    CreatedAt time.Time
    Projects  []Project
}
```

**Struct tags** (the backtick strings after each field) — extra metadata read by libraries at runtime via Go's reflection system.

- `` `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"` `` — tells GORM: this is the primary key column, its Postgres type is `uuid`, and if no value is given at insert, Postgres calls `gen_random_uuid()` to fill it. We also set it from Go (with `uuid.NewString()`) before inserting, so the default is a safety net.
- `` `gorm:"unique;not null"` `` — `UNIQUE` constraint (no two rows with same email) and `NOT NULL`.
- `CreatedAt time.Time` — GORM special field: auto-filled with `time.Now()` on insert if zero.
- `Projects []Project` — GORM association: "a user has many projects". This loads when you call `db.Preload("Projects").Find(...)` — not auto-loaded.

### Project struct

```go
type Project struct {
    ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    Name      string    `gorm:"not null"`
    UserID    string    `gorm:"type:uuid;not null"`
    CreatedAt time.Time
    Jobs      []Job
}
```

`UserID string` — the foreign key. GORM infers this belongs to `User` because there is a `User` type in the same module and the field name follows the convention `TypeID`. When you query projects filtered by `user_id`, GORM generates `WHERE user_id = ?`.

### Job struct

```go
type Job struct {
    ID         string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    Type       string    `gorm:"not null"`
    Payload    string    `gorm:"type:jsonb;not null"`
    Status     string    `gorm:"not null;default:'queued'"`
    ExecuteAt  time.Time `gorm:"index"`
    Duration   int64     // in milliseconds
    ProjectID  string    `gorm:"type:uuid;not null"`
    MaxRetries int       `gorm:"not null;default:3"`
    RetryCount int       `gorm:"not null;default:0"`
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

- `Payload string` stored as `jsonb` — the actual JSON data is kept as a string in Go but as native JSONB in Postgres (binary JSON, indexable).
- `ExecuteAt time.Time \`gorm:"index"\`` — indexed for fast lookup when implementing the scheduler (not yet used).
- `Duration int64` — milliseconds the job took; filled after completion.
- `MaxRetries int \`gorm:"default:3"\`` — default applied by Postgres if Go sends zero value. We also default it in code.
- `UpdatedAt time.Time` — GORM special: auto-updated on every `Save`/`Updates` call. Critical for the Reaper to detect stuck jobs.

---

## 7. internal/api/api.go

**File:** `internal/api/api.go`

```go
type API struct {
    db     *gorm.DB
    rdb    *redis.Client
    logger *zap.Logger
}
```

The **central API object**. All HTTP handlers are methods on this struct so they share access to the database, Redis, and logger without using global variables.

`*gorm.DB` — a pointer to a GORM database connection pool. GORM manages a pool of actual Postgres connections internally.

`*redis.Client` — a pointer to the Redis client. Also connection-pooled internally.

`*zap.Logger` — structured logger. Lowercase field names (`db`, `rdb`, `logger`) mean they are **unexported** — only code within the `api` package can access them directly.

```go
func New(db *gorm.DB, rdb *redis.Client, logger *zap.Logger) *API {
    return &API{
        db:     db,
        rdb:    rdb,
        logger: logger,
    }
}
```

A **constructor function** — Go has no `new` keyword for custom init; by convention you write `NewXxx` functions. Returns `*API` (pointer) so callers share one instance.

---

## 8. internal/api/routes.go

**File:** `internal/api/routes.go`

```go
func NewRouter(
    mw *middleware.Middleware,
    registerHandler http.HandlerFunc,
    ...
) http.Handler {
```

Returns `http.Handler` — the standard Go interface for anything that can serve HTTP. `http.Handler` has one method: `ServeHTTP(ResponseWriter, *Request)`.

`http.HandlerFunc` is a function type that satisfies `http.Handler`. You can pass any function with the right signature as an `http.HandlerFunc`.

```go
    r := chi.NewRouter()
```
Creates a new chi router. Under the hood this is a tree-based router that matches URL patterns.

```go
    r.Post("/api/v1/register", registerHandler)
    r.Post("/api/v1/login",    loginHandler)
```
Register `POST` handlers on these paths. No middleware; public endpoints.

```go
    r.Handle("/metrics", promhttp.Handler())
```
`r.Handle` accepts any `http.Handler` (not just a function). `promhttp.Handler()` returns the Prometheus exposition handler — it formats all registered metrics as text on `GET /metrics`.

```go
    r.Group(func(r chi.Router) {
        r.Use(mw.APIKeyAuth, mw.RateLimit)
        r.Post("/api/v1/projects", createProjectHandler)
        ...
    })
```
`r.Group` creates a sub-router with its own middleware stack. The anonymous function receives the sub-router as `r` (shadows outer `r` inside the closure — same variable name, different scope).

`r.Use(mw.APIKeyAuth, mw.RateLimit)` — registers middleware **in order**. Every request to routes inside this group first passes through `APIKeyAuth`, then `RateLimit`, then the actual handler.

**Middleware chain:** Request → APIKeyAuth → RateLimit → Handler → Response.

```go
        r.Get("/api/v1/job/status/{jobID}", statusHandler)
```
`{jobID}` is a **URL parameter**. chi extracts it and makes it available via `chi.URLParam(r, "jobID")` inside the handler.

---

## 9. internal/api/user.go

**File:** `internal/api/user.go`

### AuthRequest / AuthResponse

```go
type AuthRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
    APIKey   string `json:"api_key"`
}
```
`json:"email"` — struct tag telling `encoding/json` to map JSON key `"email"` ↔ Go field `Email`. Without this tag, the JSON key would be `"Email"` (capitalised). This controls both encoding (Go → JSON) and decoding (JSON → Go).

### RegisterHandler

```go
func (a *API) RegisterHandler(w http.ResponseWriter, r *http.Request) {
```
`w http.ResponseWriter` — the object you write your response to (status codes, headers, body).  
`r *http.Request` — everything about the incoming request (URL, headers, body, context).

```go
    var req AuthRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }
```
`json.NewDecoder(r.Body)` — creates a streaming JSON decoder reading from the request body. `.Decode(&req)` fills the `req` struct. `&req` passes a pointer so Decode can modify it.

If decoding fails (malformed JSON, wrong types), `err != nil` is true. `http.Error` writes the error string as plain text with the given HTTP status and returns. `return` exits the handler.

```go
    hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
```
`[]byte(req.Password)` — converts the password string to a byte slice, because `bcrypt` works with bytes.  
`bcrypt.DefaultCost` — the work factor (12 by default). Higher = slower to hash = harder to brute-force.  
Returns a hash string like `$2a$12$...`.

```go
    user := models.User{
        ID:        uuid.NewString(),
        Email:     req.Email,
        Password:  string(hash),
        APIKey:    uuid.NewString(),
        CreatedAt: time.Now(),
    }
```
Creates a `User` value with explicit field names. `uuid.NewString()` generates a random UUID string like `"550e8400-e29b-41d4-a716-446655440000"`. The API key is a second random UUID — it's the bearer token the user will use.

```go
    if err := a.db.Create(&user).Error; err != nil {
```
`a.db.Create(&user)` — GORM generates `INSERT INTO users (id, email, password, api_key, created_at) VALUES (?, ?, ?, ?, ?)` and executes it. Returns a `*gorm.DB` whose `.Error` field holds any error.

Short `if err :=` syntax: declare `err` inside the if condition, check it. `err` only lives inside the if block.

```go
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(AuthResponse{APIKey: user.APIKey})
```
Set response header first. `json.NewEncoder(w).Encode(...)` serialises the struct to JSON and writes it directly to `w`. Note: status 200 is implicit if you never call `w.WriteHeader(...)`.

### LoginHandler

```go
    if err := a.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
        http.Error(w, "invalid email or password", http.StatusUnauthorized)
        return
    }
```
`Where("email = ?", req.Email)` — parameterised query (prevents SQL injection). `.First(&user)` loads the first matching row into `user`. If no row: `gorm.ErrRecordNotFound`. We intentionally return the same error message for "not found" and "wrong password" — this is standard security practice; don't tell attackers whether the email exists.

```go
    if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
        http.Error(w, "invalid email or password", http.StatusUnauthorized)
        return
    }
```
`bcrypt.CompareHashAndPassword` re-hashes the input password and compares to the stored hash. Returns `nil` if matching, an error if not. We check `!= nil` (error = wrong password).

---

## 10. internal/api/project.go

**File:** `internal/api/project.go`

### CreateProjectHandler

```go
    if strings.TrimSpace(req.Name) == "" {
```
`strings.TrimSpace` removes leading and trailing whitespace. Prevents names like `"   "` (all spaces) from being accepted.

```go
    project := models.Project{
        ID:        uuid.NewString(),
        Name:      strings.TrimSpace(req.Name),
        UserID:    user.ID,
        CreatedAt: time.Now(),
    }
```
`user.ID` comes from `middleware.GetUser(r)` — the authenticated user injected by the auth middleware.

```go
    w.WriteHeader(http.StatusCreated)
```
Explicitly sets the HTTP response status to `201 Created`. Must be called **before** writing the body.

### loadOwnedProject

```go
func (a *API) loadOwnedProject(w http.ResponseWriter, user models.User, projectID string) (*models.Project, bool) {
```
Returns two values: `*models.Project` (the loaded project, or nil) and `bool` (true = success, false = error was written to `w`).

This pattern lets callers do:
```go
if _, ok := a.loadOwnedProject(w, user, req.ProjectID); !ok {
    return  // response already written by loadOwnedProject
}
```

```go
    err := a.db.First(&project, "id = ? AND user_id = ?", projectID, user.ID).Error
```
`First` with two `?` placeholders and two arguments. GORM fills them in order. This queries `WHERE id = projectID AND user_id = user.ID`. If the project exists but belongs to a different user, no row is returned (treated as forbidden, not "not found" — same security principle as login).

```go
    if errors.Is(err, gorm.ErrRecordNotFound) {
```
`errors.Is` checks whether `err` is (or wraps) `gorm.ErrRecordNotFound`. This is Go's sentinel error checking — more robust than `err == gorm.ErrRecordNotFound` because it works through error wrapping chains.

---

## 11. internal/api/job.go

**File:** `internal/api/job.go`

### SubmitRequest

```go
type SubmitRequest struct {
    ProjectID string                 `json:"project_id"`
    Type      string                 `json:"type"`
    Payload   map[string]interface{} `json:"payload"`
    JobID     string                 `json:"job_id"`
}
```
`map[string]interface{}` — a map with string keys and values of **any type**. This is Go's way of representing an arbitrary JSON object. `interface{}` (or its alias `any` in newer Go) means "any type". When you decode JSON into this, strings become `string`, numbers become `float64`, booleans become `bool`, nested objects become `map[string]interface{}`.

### SubmitHandler — step by step

```go
    user, ok := middleware.GetUser(r)
    if !ok {
        http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
        return
    }
```
`middleware.GetUser(r)` retrieves the authenticated user from the request context. If somehow called without auth middleware, `ok` is false. `http.StatusText(401)` returns the string `"Unauthorized"`.

```go
    if err := jobs.ValidateSubmit(req.ProjectID, req.Type, req.Payload); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
```
`err.Error()` calls the `Error() string` method on the error interface — returns the human-readable message. This is what gets sent as the response body.

```go
    if _, ok := a.loadOwnedProject(w, user, req.ProjectID); !ok {
        return
    }
```
`_` discards the project (we don't need it here, we just need to know it belongs to the user). `!ok` — if the project doesn't exist or doesn't belong to the user, `loadOwnedProject` already wrote a `403 Forbidden` to `w`. We just return.

```go
    payloadJSON, err := json.Marshal(req.Payload)
```
`json.Marshal` converts a Go value to JSON bytes. We stored the payload as `map[string]interface{}` for decoding, but now we re-encode it to a canonical JSON string to store in Postgres.

```go
    job := models.Job{
        ID:        uuid.NewString(),
        ...
        Status:    models.StatusQueued,
        ...
    }
```
We generate the ID in Go (not relying on Postgres default) so we can immediately use it for the Redis `LPUSH` below.

```go
    queueName := heuristics.GetQueue(req.Type)
    if err := a.rdb.LPush(r.Context(), queueName, job.ID).Err(); err != nil {
```
`r.Context()` — the request's context. If the client disconnects, this context is cancelled, which cancels the Redis call.  
`LPush(ctx, key, value)` — Redis command: Left-Push. Inserts `job.ID` at the head of the list named `queueName`. Returns a Redis result chain; `.Err()` extracts just the error.

```go
    w.WriteHeader(http.StatusAccepted)  // 202
    json.NewEncoder(w).Encode(SubmitResponse{JobID: job.ID})
```
`202 Accepted` is the correct status for async operations: "I received your request and will process it, but haven't done so yet."

### StatusHandler

```go
    jobID := chi.URLParam(r, "jobID")
```
Extracts the `{jobID}` path parameter that chi parsed.

```go
    if err := a.db.First(&job, "id = ?", jobID).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            http.Error(w, "job not found", http.StatusNotFound)
            return
        }
```
Two-tier error check: not found vs. any other DB error. Different responses for different failure modes.

### ListHandler

```go
    var jobs []models.Job
    a.db.Where("project_id = ?", projectID).
        Order("created_at desc").
        Find(&jobs)
```
`var jobs []models.Job` — declares a nil slice of `Job`.  
Method chaining on GORM: each method returns `*gorm.DB` for the next call.  
`.Order("created_at desc")` — SQL `ORDER BY created_at DESC`.  
`.Find(&jobs)` — executes the query, populates `jobs`. If no rows, `jobs` stays empty (not an error).

Error is intentionally not checked here (a known gap — should be `if err := a.db...Find(&jobs).Error; err != nil`).

---

## 12. internal/middleware/auth.go

**File:** `internal/middleware/auth.go`

### The type and constant

```go
type ctxKey string
```
Defines a **new named type** based on `string`. This is a technique to avoid context key collisions. If you used a plain `string` as a context key, any package using the same string `"user"` could accidentally read your value. By creating a unique named type `ctxKey`, even `ctxKey("user")` and `string("user")` are different keys.

```go
const userCtxKey = ctxKey("user")
```
The actual key value. Typed as `ctxKey`, not `string`.

### APIKeyAuth middleware

```go
func (m *Middleware) APIKeyAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
```

**Middleware signature:** takes the next handler, returns a handler. When registered with `r.Use(...)`, chi wraps your handler: incoming request → APIKeyAuth → ... → your handler.

`http.HandlerFunc(func(...))` — converts an anonymous function to the `http.Handler` interface. The anonymous function captures `next` from the outer scope (closure).

```go
        auth := r.Header.Get("Authorization")
        apiKey := strings.TrimPrefix(auth, "Bearer ")
```
`r.Header.Get("Authorization")` returns e.g. `"Bearer abc123"`.  
`strings.TrimPrefix(s, prefix)` removes the prefix if present; returns `s` unchanged if the prefix is not there. Result: `"abc123"`.

```go
        var user models.User
        if err := m.DB.Where("api_key = ?", apiKey).First(&user).Error; err != nil {
            if err == gorm.ErrRecordNotFound {
```
Note: uses `==` not `errors.Is` here — a minor inconsistency vs the rest of the codebase, but works because GORM returns the sentinel directly at this level.

```go
        ctx := context.WithValue(r.Context(), userCtxKey, user)
        next.ServeHTTP(w, r.WithContext(ctx))
```
`context.WithValue(parent, key, value)` — creates a new context derived from the parent with an extra key-value pair. The user struct is now embedded in the request context.

`r.WithContext(ctx)` — creates a new `*http.Request` with the new context attached. Immutable — doesn't modify `r`.

`next.ServeHTTP(w, r.WithContext(ctx))` — calls the next handler (the actual route handler or next middleware) with the enriched request.

### GetUser

```go
func GetUser(r *http.Request) (models.User, bool) {
    user, ok := r.Context().Value(userCtxKey).(models.User)
    return user, ok
}
```
`.Value(userCtxKey)` returns `interface{}` (any value stored under that key).  
`.(models.User)` is a **type assertion** — extracts the concrete `models.User` value from the interface. The two-value form `user, ok := ....(models.User)` sets `ok = false` if the value is not a `models.User` (safe — doesn't panic).

---

## 13. internal/middleware/ratelimit.go

**File:** `internal/middleware/ratelimit.go`

### Middleware struct

```go
type Middleware struct {
    DB    *gorm.DB
    Cache *cache.Cache
}
```
Exported struct (capital letter). The `Cache` field is an in-memory TTL cache from `go-cache`. It maps `string → *rate.Limiter` (though stored as `interface{}`).

### getLimiter

```go
func (m *Middleware) getLimiter(apiKey string) *rate.Limiter {
    limiter, found := m.Cache.Get(apiKey)
    if !found {
        limiter = rate.NewLimiter(5, 10) // 5 req/sec, burst 10
        m.Cache.Set(apiKey, limiter, 1*time.Hour)
    }
    return limiter.(*rate.Limiter)
}
```

`m.Cache.Get(apiKey)` — returns `(interface{}, bool)`. `found` is `true` if the key exists and hasn't expired.

`rate.NewLimiter(r, b)` — token bucket algorithm:
- `r = 5` — refill rate: 5 tokens per second added back.
- `b = 10` — bucket size: can hold up to 10 tokens (allows bursts of 10 before refilling).

Each request consumes 1 token. At 5 req/s steady state, 5 tokens consumed and 5 refilled = sustainable. A burst of 10 can happen before throttling.

`m.Cache.Set(apiKey, limiter, 1*time.Hour)` — stores the limiter for this API key for 1 hour. After that it expires and a fresh limiter is created. This prevents unbounded memory growth.

`limiter.(*rate.Limiter)` — type assertion back from `interface{}` to the concrete type. One-value form — panics if the type doesn't match. Safe here because we're the ones who set it.

### RateLimit middleware

```go
        if !limiter.Allow() {
            http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
```
`limiter.Allow()` — attempts to consume 1 token. Returns `true` if token was consumed (request is allowed), `false` if bucket is empty (rate exceeded). HTTP 429 Too Many Requests.

---

## 14. internal/jobs/validator.go

**File:** `internal/jobs/validator.go`

### Sentinel errors

```go
var (
    ErrProjectIDRequired = errors.New("project_id is required")
    ErrJobTypeRequired   = errors.New("type is required")
    ErrUnknownJobType    = errors.New("unknown job type")
    ErrPayloadRequired   = errors.New("payload is required")
)
```
`var ( ... )` — block of package-level variables.

`errors.New("text")` — creates a new error value with that message. These are **sentinel errors** — specific error instances that callers can check for:
```go
if errors.Is(err, jobs.ErrUnknownJobType) { ... }
```
This is the Go equivalent of typed exceptions. Instead of `catch (UnknownJobTypeException)`, you test `errors.Is`.

### ValidateSubmit

```go
func ValidateSubmit(projectID, jobType string, payload map[string]interface{}) error {
```
Three parameters; the first two are both `string` (Go allows grouping same-type params: `projectID, jobType string`).

```go
    if strings.TrimSpace(projectID) == "" {
        return ErrProjectIDRequired
    }
```
Early return pattern: validate each field in sequence, return the first error found. The caller gets one specific error, not a list.

```go
    if _, ok := config.AllowedJobTypes[jobType]; !ok {
        return ErrUnknownJobType
    }
```
Map lookup: `map[key]` returns `(value, bool)`. We don't need the value (`_`), just whether the key exists (`ok`). `!ok` = key not in map.

```go
    if payload == nil {
        return ErrPayloadRequired
    }
    return nil
```
For map types, `nil` is the zero value (uninitialized). An empty map `{}` is different from `nil` — `{}` passes validation.

Returns `nil` on success — idiomatic for "no error".

---

## 15. internal/jobs/enqueue.go

**File:** `internal/jobs/enqueue.go`

```go
func EnqueueJobID(rdb *redis.Client, jobType, jobID string) error {
    queueName := heuristics.GetQueue(jobType)
    return rdb.LPush(context.Background(), queueName, jobID).Err()
}
```

Short, focused function. Takes a Redis client, the job type (to determine which queue), and the job ID.

`context.Background()` — a root context with no deadline or cancellation. Used here because this is a utility function that doesn't have a request context to propagate.

`rdb.LPush(ctx, key, values...)` — Redis `LPUSH`. Returns a `*redis.IntCmd` (result of the command). `.Err()` extracts just the error from the result.

Why push the **ID** and not the full job JSON? Because PostgreSQL is the source of truth. Workers look up the job in Postgres, lock the row, and then process. Putting only the ID in Redis keeps the queue lightweight and avoids data consistency issues.

---

## 16. internal/heuristics/router.go

**File:** `internal/heuristics/router.go`

```go
func GetQueue(jobType string) string {
    switch jobType {
    case config.JobTypeSendEmail:
        return config.QueueHigh
    default:
        return config.QueueDefault
    }
}
```

A `switch` in Go does not fall through by default (unlike C). Each `case` is independent.

`default` handles any value not explicitly listed — here, `generate_receipt` and `summarize_text` both fall to default and go to `queue:default`.

**Why is email on `queue:high`?** Transactional email is typically user-facing and time-sensitive ("verify your email" for a new signup). PDF generation and summarization are batch/background operations that can wait. This priority mapping is the "heuristic" — it can be extended as new job types are added.

```go
// GetPriorityQueue is deprecated; use GetQueue.
func GetPriorityQueue(jobType string) string {
    return GetQueue(jobType)
}
```
A shim kept for backwards compatibility (tests or external code that referenced the old name). It just delegates. Marked deprecated in the comment so it shows up as a warning in IDEs.

---

## 17. internal/tasks/processor.go

**File:** `internal/tasks/processor.go`

### The Processor interface

```go
type Processor interface {
    Process(ctx context.Context, job models.Job) error
}
```

Every job type must implement this. A worker calls `processor.Process(ctx, job)` without knowing what type of processor it is — polymorphism.

`ctx context.Context` — allows the processor to respect cancellation. If the worker's context is cancelled (e.g. shutdown signal), a well-written processor checks `ctx.Done()` and exits early.

### The registry

```go
var processors = make(map[string]Processor)
```
`var` at package level — this map is shared by the entire process. Lowercase `processors` = unexported (only `Register` and `Get` in this package can touch it directly).

`make(map[string]Processor)` — initialises the map. In Go, a nil map panics on write. Always use `make` or a map literal `{}` to initialise.

```go
func Register(jobType string, p Processor) {
    processors[jobType] = p
}
```
Called in `main.go` at startup. Maps the string type name to the actual struct that can process it.

```go
func Get(jobType string) (Processor, error) {
    p, ok := processors[jobType]
    if !ok {
        return nil, fmt.Errorf("no processor registered for job type: %s", jobType)
    }
    return p, nil
}
```
`fmt.Errorf("...", args)` — creates a formatted error. `%s` is the string verb (inserts the jobType string).

### MockEmailSender

```go
type MockEmailSender struct{}
```
An empty struct — zero bytes. It has no fields because the mock doesn't need state. It satisfies `Processor` because it has a `Process` method.

```go
func (s *MockEmailSender) Process(ctx context.Context, job models.Job) error {
    fmt.Printf("SIMULATING: Sending email for job %s. Payload: %s\n", job.ID, job.Payload)
    if job.ID[0]%2 == 0 { // Simulate occasional error
        return errors.New("simulated SMTP connection error")
    }
    return nil
}
```
`job.ID[0]` — first byte of the UUID string. `%2 == 0` checks if it's even. This is a hack to simulate ~50% failure rate for testing the retry mechanism. In a UUID, the first character is a hex digit (0-9, a-f). The ASCII values of '0','2','4','6','8' are even; '1','3','5','7','9','a'-'f' vary. Rough 50/50.

### MockSummarizer

```go
func (s *MockSummarizer) Process(ctx context.Context, job models.Job) error {
    fmt.Printf("SIMULATING: Summarizing text for job %s. Payload: %s\n", job.ID, job.Payload)
    return nil
}
```
Always succeeds. Used to verify the full pipeline works without testing summarisation logic.

---

## 18. internal/tasks/receipt.go

**File:** `internal/tasks/receipt.go`

### ReceiptPayload

```go
type ReceiptPayload struct {
    To      string  `json:"to"`
    Item    string  `json:"item"`
    Amount  float64 `json:"amount"`
    IsPaid  bool    `json:"is_paid"`
}
```
A typed struct for parsing the payload. `float64` is the default JSON number type in Go.

### Process

```go
func (g *ReceiptGenerator) Process(ctx context.Context, job models.Job) error {
    var p ReceiptPayload
    if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
        return fmt.Errorf("failed to unmarshal receipt payload: %w", err)
    }
```
`json.Unmarshal(data []byte, v interface{})` — parses JSON into the pointer `&p`.  
`[]byte(job.Payload)` — type conversion: `string` → `[]byte`. JSON functions work on bytes.  
`%w` in `fmt.Errorf` — **wraps** the original error. Callers can unwrap it with `errors.Is` or `errors.As`.

```go
    pdf := gofpdf.New("P", "mm", "A4", "")
```
- `"P"` — Portrait orientation.
- `"mm"` — millimetres as unit.
- `"A4"` — paper size.
- `""` — empty string for font directory (uses built-in fonts).

```go
    pdf.AddPage()
    pdf.SetFont("Arial", "B", 16)    // Bold, 16pt
    pdf.Cell(40, 10, "Receipt")      // 40mm wide, 10mm tall cell with text
    pdf.Ln(20)                       // Line break, 20mm spacing
    pdf.SetFont("Arial", "", 12)     // Regular, 12pt
```

```go
    filePath := fmt.Sprintf("receipt-%s.pdf", job.ID)
    err := pdf.OutputFileAndClose(filePath)
```
Writes the PDF to a file in the current working directory. `job.ID` makes the filename unique. In production this should upload to S3 and store the URL.

```go
    if err != nil {
        return fmt.Errorf("failed to generate PDF: %w", err)
    }
```
Wraps the gofpdf error with context. This error is retryable — the worker will retry if it gets this.

---

## 19. internal/workers/pool.go

**File:** `internal/workers/pool.go`

### Pool struct

```go
type Pool struct {
    ctx           context.Context
    cancel        context.CancelFunc
    jobQueue      string
    min, max, num int
    nextWorkerID  int
    mu            sync.Mutex
    wg            sync.WaitGroup
    workers       map[int]context.CancelFunc
    ...
}
```

`context.Context` — the pool's lifetime context. When cancelled, all workers stop.  
`context.CancelFunc` — the function that cancels that context. Stored so `Shutdown()` can call it.  
`min, max, num int` — minimum workers, maximum workers, current count. Three ints in one declaration.  
`sync.Mutex` — mutual exclusion lock. Prevents two goroutines from modifying `workers`, `num`, `nextWorkerID` at the same time.  
`sync.WaitGroup` — a counter. `wg.Add(1)` when a goroutine starts, `wg.Done()` when it exits, `wg.Wait()` blocks until count reaches zero.  
`workers map[int]context.CancelFunc` — maps each worker's ID to its cancel function. Calling `cancel()` stops that specific worker.

### NewPool

```go
func NewPool(ctx context.Context, queue string, min, max int, ...) *Pool {
    pCtx, pCancel := context.WithCancel(ctx)
```
`context.WithCancel(parent)` — creates a derived context. If `parent` is cancelled, `pCtx` is also cancelled. `pCancel` cancels only `pCtx` and its children.

```go
    pool := &Pool{
        ...
        workers: make(map[int]context.CancelFunc),
        logger:  logger.With(zap.String("queue", queue)),
        ...
    }
```
`logger.With(zap.String("queue", queue))` — creates a child logger that always includes `queue=queue:high` in every log line. Avoids repeating this field.

```go
    pool.ScaleUp(min)
    return pool
```
Immediately starts `min` workers.

### ScaleUp

```go
func (p *Pool) ScaleUp(n int) {
    p.mu.Lock()
    defer p.mu.Unlock()
```
Lock the mutex before touching shared fields. `defer p.mu.Unlock()` releases the lock when `ScaleUp` returns, regardless of how it returns. Without this lock, two concurrent ScaleUp calls could corrupt `p.num`.

```go
    for i := 0; i < n && p.num < p.max; i++ {
```
Loop `n` times but stop early if at max. The `&&` short-circuits.

```go
        wCtx, wCancel := context.WithCancel(p.ctx)
        p.workers[workerID] = wCancel
        p.num++
        p.wg.Add(1)
```
Each worker gets its own cancellable context derived from the pool's context. Stored so ScaleDown can cancel individual workers.

```go
        worker := NewWorker(workerID, p.jobQueue, p.db, p.rdb, p.ai, p.metrics, p.logger)
        go func(id int) {
            defer func() {
                p.mu.Lock()
                delete(p.workers, id)
                p.mu.Unlock()
                p.wg.Done()
            }()
            worker.Loop(wCtx)
        }(workerID)
```
`go func(id int) { ... }(workerID)` — starts a goroutine. The `(workerID)` at the end **immediately calls** the anonymous function with the current `workerID` as an argument. This is important: without the argument, the goroutine would capture `workerID` by reference and all goroutines might end up with the same (last) value.

`defer func() { ... }()` — when the goroutine's function returns (worker exits), clean up: remove from map, decrement WaitGroup.

`worker.Loop(wCtx)` — blocks; the worker loops until `wCtx` is cancelled.

### ScaleDown

```go
    for id, cancel := range p.workers {
        if i >= n || p.num <= p.min {
            break
        }
        cancel()
        delete(p.workers, id)
        p.num--
        i++
    }
```
`range p.workers` — iterates map entries as `key, value` pairs. Calls `cancel()` which signals that worker's context to stop. The worker's loop detects `ctx.Done()` and exits. The `defer` inside the goroutine then removes it from the map (which we already did here — slight redundancy but safe).

### Shutdown

```go
func (p *Pool) Shutdown() {
    p.cancel()    // cancel pool context -> all worker contexts are cancelled
    p.wg.Wait()   // block until all workers have exited
}
```
Graceful shutdown: signal all workers, wait for them to finish their current job and return.

---

## 20. internal/workers/worker.go

**File:** `internal/workers/worker.go`

```go
const (
    dlqKey = "queue:dlq"
)
```
Lowercase = unexported. Only used within the `workers` package.

### Loop — the main event loop

```go
func (w *Worker) Loop(ctx context.Context) {
    w.logger.Info("worker loop started")
    for {
        select {
        case <-ctx.Done():
            w.logger.Info("worker loop stopping")
            return
        default:
            result, err := w.rdb.BRPop(ctx, 5*time.Second, w.queue).Result()
```

`for { ... }` — infinite loop.

`select { case <-ctx.Done(): ... default: ... }` — non-blocking check: if the context is already done, exit. Otherwise fall to `default` and try to get a job. Without `default`, `select` would block on `ctx.Done()` indefinitely, making it equivalent to just `<-ctx.Done()`. The `default` makes it a non-blocking peek.

`w.rdb.BRPop(ctx, timeout, key)` — **Blocking Right Pop**. Waits up to `timeout` for an item in the list, then returns. Returns `([]string, error)`:
- `[]string` = `[key, value]` — index 0 is the list name, index 1 is the value (job ID).
- `error = redis.Nil` if timeout expired with nothing.

```go
            if err != nil {
                if errors.Is(err, redis.Nil) {
                    continue // Timeout, no job received.
                }
                w.logger.Error("failed to pop job from queue", zap.Error(err))
                time.Sleep(1 * time.Second)
                continue
            }
```
`continue` — jumps back to the top of `for`, re-running the `select`.

`time.Sleep(1 * time.Second)` — if there is a real Redis error (connection problem), wait 1 second before retrying to avoid a tight error loop hammering the logs.

```go
            if len(result) < 2 {
                continue
            }
            w.processJob(ctx, result[1])
```
Defensive check: BRPop should always return 2 elements, but guard anyway.

### processJob — the core

```go
func (w *Worker) processJob(ctx context.Context, jobID string) {
    startTime := time.Now()

    tx := w.db.Begin()
    if tx.Error != nil { ... }
    defer tx.Rollback()
```
`w.db.Begin()` — opens a database transaction. Returns a new `*gorm.DB` bound to that transaction. All operations on `tx` are part of the same atomic transaction.

`defer tx.Rollback()` — if anything goes wrong and we return early without committing, the transaction is rolled back (changes undone). GORM's `Rollback()` is a no-op if the transaction was already committed — safe to defer unconditionally.

```go
    var job models.Job
    if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, "id = ?", jobID).Error; err != nil {
```
`clause.Locking{Strength: "UPDATE"}` — adds `FOR UPDATE` to the SQL query. This is a **row-level lock**: no other transaction can lock or modify this row until this transaction commits or rolls back.

**Why?** Multiple workers might receive the same job ID if the system restarts or there is a bug. With `FOR UPDATE`, only one worker acquires the lock and proceeds; others wait, then see the status is already `running` and skip.

```go
    if job.Status != models.StatusQueued {
        w.logger.Warn("job was already processed", ...)
        return // Idempotency check
    }
```
**Idempotency**: even if the job was delivered twice, only the first worker processes it. The second sees `running` or `completed` and exits cleanly.

```go
    job.Status = models.StatusRunning
    if err := tx.Save(&job).Error; err != nil { ... }
    if err := tx.Commit().Error; err != nil { ... }
```
`tx.Save(&job)` — `UPDATE jobs SET status='running', ... WHERE id=?`.  
`tx.Commit()` — persists the status change and releases the lock. Now other workers will see `running`.

After commit, we release the DB lock but continue to execute the task.

```go
    processingErr := w.executeTask(ctx, job)

    duration := time.Since(startTime).Milliseconds()
```
`time.Since(t)` returns the elapsed time as `time.Duration` (nanoseconds).  
`.Milliseconds()` converts to int64 milliseconds.

```go
    if processingErr != nil {
        w.metrics.JobFailuresTotal.WithLabelValues(w.queue, job.Type).Inc()
        w.handleFailure(ctx, job, duration)
    } else {
        if err := w.db.Model(&job).Updates(models.Job{Status: models.StatusCompleted, Duration: duration}).Error; err != nil {
```
`.Updates(models.Job{...})` — GORM only updates non-zero fields. `Status` is a string (zero value `""`), so GORM would skip it — except we're passing a full struct with `Status = "completed"`, which is non-zero. `Duration` is `int64`; zero would be skipped if the job took 0ms (rare edge case in tests).

A safer pattern would be `map[string]interface{}{"status": ..., "duration": ...}`.

```go
    w.metrics.JobDurationSeconds.WithLabelValues(w.queue, job.Type).Observe(float64(duration) / 1000)
```
`Observe(value)` — adds a value to the histogram. `float64(duration) / 1000` converts milliseconds to seconds (Prometheus convention for durations).

### handleFailure

```go
func (w *Worker) handleFailure(ctx context.Context, job models.Job, duration int64) {
    job.RetryCount++
    job.Duration = duration

    if job.RetryCount > job.MaxRetries {
```
Note: `job` is a **value** (copy), not a pointer. Incrementing `job.RetryCount` modifies the local copy; we then save it to DB explicitly.

**Branch 1 — permanent failure:**

```go
        jobData, _ := json.Marshal(job)
        if err := w.rdb.LPush(ctx, dlqKey, jobData).Err(); err != nil {
```
`json.Marshal(job)` — the full job struct as JSON bytes. `_` discards the error (if marshal fails, we log below anyway).

DLQ stores full JSON so operators can inspect the payload without querying Postgres.

```go
        var payload map[string]interface{}
        _ = json.Unmarshal([]byte(job.Payload), &payload)
        go w.ai.HandleDLQWithAI(ctx, job.ID, job.Type, payload, job.RetryCount)
```
`go` — fire and forget. AI summary is best-effort; we don't block the worker on it.

**Branch 2 — retryable failure:**

```go
        if err := w.rdb.LPush(ctx, w.queue, job.ID).Err(); err != nil {
```
Re-push only the **job ID** to the same worker pool's queue (not through heuristics). This ensures the retry goes to the same priority level.

---

## 21. internal/workers/autoscaler.go

**File:** `internal/workers/autoscaler.go`

### AutoScaler struct

```go
type AutoScaler struct {
    pool        *Pool
    rdb         *redis.Client
    ...
    interval    time.Duration
    scaleUp     int64
    scaleDown   int64
    scaleUpInc  int
    scaleDownInc int
}
```
`time.Duration` — Go's type for time intervals. Internally nanoseconds as int64.

Configuration values:
- `interval = 5s` — check every 5 seconds.
- `scaleUp = 20` — if queue has > 20 jobs, add workers.
- `scaleDown = 5` — if queue has < 5 jobs, remove workers.
- `scaleUpInc = 2` — add 2 workers per trigger.
- `scaleDownInc = 1` — remove 1 worker per trigger.

### Run

```go
func (as *AutoScaler) Run(ctx context.Context) {
    ticker := time.NewTicker(as.interval)
    defer ticker.Stop()
```
`time.NewTicker(d)` — fires on a channel (`ticker.C`) every `d`. Unlike `time.Sleep`, it doesn't drift (accounts for processing time).

`defer ticker.Stop()` — stops the ticker goroutine when `Run` returns, preventing a goroutine leak.

```go
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            numWorkers, queueName := as.pool.GetStats()
            length, err := as.rdb.LLen(ctx, queueName).Result()
```
`LLen(ctx, key)` — Redis `LLEN`: returns the length of the list (number of pending jobs).

```go
            as.metrics.QueueLength.WithLabelValues(queueName).Set(float64(length))
```
`Set(float64(length))` — updates the Prometheus gauge to current queue depth. Visible in Grafana.

```go
            if length > as.scaleUp {
                as.pool.ScaleUp(as.scaleUpInc)
            } else if length < as.scaleDown {
                as.pool.ScaleDown(as.scaleDownInc)
            }
```
Hysteresis band: scale up above 20, scale down below 5. The gap (5-20) is the steady band where nothing changes — prevents flapping.

---

## 22. internal/workers/reaper.go

**File:** `internal/workers/reaper.go`

The Reaper fixes **stuck jobs**: jobs with `status='running'` that haven't been updated in over an hour. This happens when a worker crashes mid-job (the `defer tx.Rollback()` didn't run because the process died hard).

### NewReaper

```go
    logger: logger.With(zap.String("component", "reaper")),
    interval:    5 * time.Minute,
    maxStuckAge: 1 * time.Hour,
```
`5 * time.Minute` — `time.Minute` is a constant (60 billion nanoseconds), multiplied by 5.

### reapStuckJobs

```go
    stuckTime := time.Now().Add(-r.maxStuckAge)
    if err := r.db.Where("status = ? AND updated_at < ?", models.StatusRunning, stuckTime).Find(&stuckJobs).Error; err != nil {
```
`time.Now().Add(-1 * time.Hour)` — 1 hour ago. The `Add` method takes a `Duration`; negative means go back in time.

SQL equivalent: `WHERE status = 'running' AND updated_at < NOW() - INTERVAL '1 hour'`.

```go
    for _, job := range stuckJobs {
        tx := r.db.Begin()
        job.Status = models.StatusQueued
        if err := tx.Save(&job).Error; err != nil {
            tx.Rollback()
            ...
            continue
        }

        queueName := heuristics.GetQueue(job.Type)
        if err := r.rdb.LPush(ctx, queueName, job.ID).Err(); err != nil {
            tx.Rollback()
            ...
            continue
        }

        if err := tx.Commit().Error; err != nil { ... }
        r.metrics.JobsReapedTotal.WithLabelValues(queueName).Inc()
```

**Why the explicit rollbacks?** Unlike `processJob`, the reaper uses `tx.Rollback()` inside the loop, not `defer`. This is because if Redis push fails after the DB update, we need to undo the DB update. With `defer`, a rollback would happen at function return, not per-job. Manual control is needed here.

`for _, job := range stuckJobs` — `_` discards the index (we don't need `0, 1, 2...`), `job` is a copy of each element. Since `job.Status = models.StatusQueued` modifies a copy, the original slice is unchanged — fine here because we're saving via `tx.Save`.

---

## 23. internal/ai/client.go

**File:** `internal/ai/client.go`

```go
type AI struct {
    rdb *redis.Client
    // Potentially an LLM client would go here in a real app
}

func New(rdb *redis.Client) *AI {
    return &AI{rdb: rdb}
}
```
Minimal struct. The Redis client is needed to store summaries. The LLM client field is planned but not implemented — the comment documents intent.

---

## 24. internal/ai/summarizer.go

**File:** `internal/ai/summarizer.go`

```go
func SummarizeFailure(jobID, jobType string, payload map[string]interface{}, retries int) string {
    prompt := fmt.Sprintf(
        "Job %s of type '%s' failed after %d retries. Payload: %v. Why?",
        jobID, jobType, retries, payload,
    )
    // TODO: call real LLM client here
    summary := "LLM says: " + prompt
    return summary
}
```
`%v` — Go's default formatting verb. For `map[string]interface{}` it prints something like `map[to:user@example.com]`.

This is a stub that constructs what would become an LLM prompt and returns a fake response. In Phase 3 this becomes an actual HTTP call to OpenAI or a local model.

```go
func (a *AI) HandleDLQWithAI(ctx context.Context, ...) {
    summary := SummarizeFailure(...)
    log.Printf("🧠 AI Summary for job %s: %s\n", jobID, summary)
    key := fmt.Sprintf("dlq_summary:%s", jobID)
    if err := a.rdb.Set(ctx, key, summary, 0).Err(); err != nil {
        log.Println("❌ Failed to save AI summary:", err)
    }
}
```
`a.rdb.Set(ctx, key, value, expiration)` — Redis `SET`. `0` means no expiry (persists until deleted).  
`log.Printf` vs `zap.Logger.Info` — note this file uses the standard `log` package not zap. Minor inconsistency (future cleanup: wire zap through).

---

## 25. internal/monitoring/prometheus.go

**File:** `internal/monitoring/prometheus.go`

### What is Prometheus?

Prometheus is a monitoring system that scrapes metrics from your app at regular intervals (e.g. every 15s). Your app exposes a `/metrics` HTTP endpoint. Prometheus pulls it. Grafana visualises the data.

### Metric types used

| Type | What it does | Used for |
|------|--------------|----------|
| `CounterVec` | Only goes up; never resets | Total jobs processed/failed |
| `GaugeVec` | Goes up and down | Current workers, queue depth |
| `HistogramVec` | Tracks value distribution in buckets | Job duration |

### Struct and registration

```go
type Metrics struct {
    JobsProcessedTotal  *prometheus.CounterVec
    JobFailuresTotal    *prometheus.CounterVec
    JobsReapedTotal     *prometheus.CounterVec
    JobDurationSeconds  *prometheus.HistogramVec
    ActiveWorkers       *prometheus.GaugeVec
    QueueLength         *prometheus.GaugeVec
}
```

```go
JobsProcessedTotal: promauto.NewCounterVec(
    prometheus.CounterOpts{
        Namespace: "jobqueue",
        Name:      "jobs_processed_total",
        Help:      "Total number of jobs processed, partitioned by status.",
    },
    []string{"queue", "status"},
),
```
`promauto` — auto-registration. Creates the metric AND registers it with Prometheus's default registry.  
`Namespace: "jobqueue"` — prefix. Full metric name: `jobqueue_jobs_processed_total`.  
`[]string{"queue", "status"}` — label names. Each combination of label values is a separate time series.

### Using the metrics

```go
// Increment a counter with specific label values
w.metrics.JobsProcessedTotal.WithLabelValues("queue:high", "completed").Inc()

// Set a gauge
p.metrics.ActiveWorkers.WithLabelValues("queue:high").Set(float64(p.num))

// Observe a histogram value
w.metrics.JobDurationSeconds.WithLabelValues("queue:high", "send_email").Observe(0.45)
```

`WithLabelValues(...)` returns the specific metric instance for those label values, creating it lazily if it doesn't exist. Label order must match the declaration order.

### Histogram buckets

```go
Buckets: prometheus.LinearBuckets(0.1, 0.1, 10),
// = [0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0]
```
10 buckets from 0.1s to 1.0s in 0.1s steps. A job taking 0.35s would be counted in the 0.3, 0.4, 0.5, ... 1.0 buckets (Prometheus histograms are cumulative).

---

## 26. cmd/server/main.go — The Entry Point

**File:** `cmd/server/main.go`

This file wires every other file together. Reading it top-to-bottom traces the startup sequence.

```go
package main

func main() {
```
`package main` + `func main()` = the executable entry point. Go finds this function and runs it.

### Step 1 — Logger

```go
    logger, err := zap.NewDevelopment()
    if err != nil {
        log.Fatalf("can't initialize zap logger: %v", err)
    }
    defer logger.Sync()
```
`zap.NewDevelopment()` — creates a console-friendly logger with colour, stack traces on errors, DEBUG level enabled. For production: `zap.NewProduction()` (JSON output, no debug).

`log.Fatalf(...)` — used before zap is set up. Prints to stderr and calls `os.Exit(1)`.

`defer logger.Sync()` — flushes any buffered log entries when main returns (e.g. on graceful shutdown). Without this, last log lines may be lost.

### Step 2 — Register task processors

```go
    tasks.Register("send_email", &tasks.MockEmailSender{})
    tasks.Register("generate_receipt", &tasks.ReceiptGenerator{})
    tasks.Register("summarize_text", &tasks.MockSummarizer{})
```
Must happen **before** workers start. Workers call `tasks.Get(job.Type)` — if not registered, the job fails permanently.

`&tasks.MockEmailSender{}` — creates an instance and takes its address (pointer). The `Processor` interface requires pointer receivers in some cases. Using `&` is safe and conventional.

### Step 3 — Config

```go
    cfg, err := config.Load()
    if err != nil {
        logger.Fatal("failed to load config", zap.Error(err))
    }
```
`logger.Fatal(msg, fields...)` — logs the message with level FATAL and calls `os.Exit(1)`. No recovery.

`zap.Error(err)` — a zap field: adds the error as `"error": "..."` in the structured log output.

### Step 4 — Shutdown context

```go
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()
```
`signal.NotifyContext` — creates a context that is cancelled when the process receives `SIGINT` (Ctrl+C) or `SIGTERM` (Docker/K8s stop).

`syscall.SIGINT` = interrupt signal (2), `syscall.SIGTERM` = termination signal (15).

`stop()` — deregisters the signal handler. Called on main's return.

This context is passed everywhere — to workers, Redis calls, etc. When Ctrl+C is pressed, `ctx` is cancelled, which cascades through the whole system.

### Step 5 — Database

```go
    db, err := gorm.Open(postgres.Open(cfg.PostgresDSN), &gorm.Config{})
```
`postgres.Open(dsn)` — creates a GORM dialector (DB driver) for PostgreSQL.  
`gorm.Open(dialector, config)` — opens the connection pool. `&gorm.Config{}` passes an empty config (uses defaults).

```go
    if err := db.AutoMigrate(&models.User{}, &models.Project{}, &models.Job{}); err != nil {
        logger.Fatal("auto-migrate failed", zap.Error(err))
    }
```
`AutoMigrate` — inspects each struct's tags and creates/alters the corresponding table to match. Safe to run on every startup: it only adds columns/indexes, never drops them.

`&models.User{}` — passes a pointer to a zero-value User struct. GORM only uses the type information (via reflection), not the actual values.

### Step 6 — Redis

```go
    redisOpts, err := redis.ParseURL(cfg.RedisURL)
```
`redis.ParseURL("redis://localhost:6379/0")` — parses host, port, database number (0-15), password, TLS settings from a URL.

```go
    rdb := redis.NewClient(redisOpts)
    if _, err := rdb.Ping(ctx).Result(); err != nil {
        logger.Fatal("failed to connect to redis", zap.Error(err))
    }
```
`Ping` verifies the connection. Fail fast — no point starting workers if Redis is unreachable.

### Step 7 — Dependencies

```go
    metrics := monitoring.NewMetrics()
    aiClient := ai.New(rdb)
    apiHandler := api.New(db, rdb, logger)
    mw := &middleware.Middleware{
        DB:    db,
        Cache: cache.New(5*time.Minute, 10*time.Minute),
    }
```
`cache.New(defaultExpiry, cleanupInterval)` — `go-cache` in-memory store. Expires entries after 5 minutes by default; runs a cleanup goroutine every 10 minutes to evict expired entries.

### Step 8 — Worker pools

```go
    highPriorityPool := workers.NewPool(ctx, "queue:high", 1, 10, db, rdb, aiClient, metrics, logger)
    defaultPool := workers.NewPool(ctx, "queue:default", 1, 10, db, rdb, aiClient, metrics, logger)
```
Two pools, each starting with 1 worker, scaling to max 10. Immediately spawn their workers in goroutines.

```go
    highPriorityScaler := workers.NewAutoScaler(highPriorityPool, rdb, metrics, logger)
    defaultScaler := workers.NewAutoScaler(defaultPool, rdb, metrics, logger)
    reaper := workers.NewReaper(db, rdb, metrics, logger)

    go highPriorityScaler.Run(ctx)
    go defaultScaler.Run(ctx)
    go reaper.Run(ctx)
```
Three background goroutines started. They block internally on `ticker.C` and `ctx.Done()`.

### Step 9 — Router

```go
    router := api.NewRouter(
        mw,
        apiHandler.RegisterHandler,
        apiHandler.LoginHandler,
        apiHandler.CreateProjectHandler,
        apiHandler.ListProjectsHandler,
        apiHandler.SubmitHandler,
        apiHandler.StatusHandler,
        apiHandler.ListHandler,
    )
```
Passes method values: `apiHandler.RegisterHandler` is equivalent to `func(w, r) { apiHandler.RegisterHandler(w, r) }`. Go supports first-class functions — methods can be passed as values.

### Step 10 — HTTP server

```go
    srv := &http.Server{
        Addr:    fmt.Sprintf(":%s", cfg.Port),
        Handler: router,
    }

    go func() {
        logger.Info("starting server", zap.String("port", cfg.Port))
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logger.Fatal("failed to start server", zap.Error(err))
        }
    }()
```
`fmt.Sprintf(":%s", cfg.Port)` — produces `":8080"`. The colon prefix means "listen on all interfaces, port 8080".

`srv.ListenAndServe()` — blocks forever serving requests. Started in a goroutine so `main` can proceed to wait for shutdown.

`err != http.ErrServerClosed` — when `srv.Shutdown()` is called gracefully, `ListenAndServe` returns `http.ErrServerClosed`. That's expected; don't log it as an error.

### Step 11 — Graceful shutdown

```go
    <-ctx.Done()
```
Blocks until the shutdown context is cancelled (Ctrl+C or SIGTERM). Channel receive blocks until something sends or the channel closes. `ctx.Done()` returns a channel that closes on cancellation.

```go
    logger.Info("shutting down server gracefully")

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := srv.Shutdown(shutdownCtx); err != nil {
        logger.Error("server shutdown failed", zap.Error(err))
    }
```
`context.WithTimeout(parent, duration)` — creates a context that auto-cancels after 5 seconds. Used so `srv.Shutdown` has a deadline: wait up to 5s for in-flight requests to finish, then force-close.

`defer cancel()` — always release the timeout context even if Shutdown returns early.

```go
    highPriorityPool.Shutdown()
    defaultPool.Shutdown()
```
Cancels pool contexts → workers detect `ctx.Done()` → finish current job → goroutines exit → `wg.Wait()` unblocks → `Shutdown()` returns.

```go
    logger.Info("shutdown complete")
}
```
`main` returns → program exits cleanly.

---

## 27. Test Files

### internal/heuristics/router_test.go

```go
package heuristics

import (
    "testing"
    "jobqueue/internal/config"
    "github.com/stretchr/testify/assert"
)
```
`package heuristics` (same as the code being tested, not `heuristics_test`) — this is a **white-box test**: can access unexported names.

```go
func TestGetQueue(t *testing.T) {
    assert.Equal(t, config.QueueHigh, GetQueue(config.JobTypeSendEmail))
    assert.Equal(t, config.QueueDefault, GetQueue(config.JobTypeGenerateReceipt))
    assert.Equal(t, config.QueueDefault, GetQueue(config.JobTypeSummarizeText))
    assert.Equal(t, config.QueueDefault, GetQueue("unknown_type"))
}
```
`assert.Equal(t, expected, actual)` — fails the test if not equal, prints a diff. `t *testing.T` is Go's test handle, passed to everything.

### internal/jobs/validator_test.go

```go
func TestValidateSubmit(t *testing.T) {
    validPayload := map[string]interface{}{"to": "a@b.com"}

    assert.NoError(t, ValidateSubmit("proj-1", config.JobTypeSendEmail, validPayload))
    assert.ErrorIs(t, ValidateSubmit("", ...), ErrProjectIDRequired)
```
`assert.ErrorIs(t, err, target)` — verifies `errors.Is(err, target)` is true. Tests that the exact sentinel error is returned.

### internal/jobs/enqueue_test.go

```go
func TestEnqueueJobID(t *testing.T) {
    redisURL := os.Getenv("REDIS_URL")
    if redisURL == "" {
        t.Skip("REDIS_URL not set")
    }
```
`t.Skip(reason)` — marks the test as skipped (not failed) and stops executing it. This is an **integration test** that requires a real Redis — skip gracefully in CI if not available.

```go
    require.NoError(t, rdb.Del(ctx, queueName).Err())
```
`require` vs `assert`: `require` stops the test immediately on failure (`t.FailNow()`), `assert` marks failure but continues. Use `require` when subsequent steps would panic if this fails.

---

## 28. End-to-End Flow

Here is the complete journey of a single job through every file:

### Phase 1: Submit

```
Client → POST /api/v1/job/submit
         Body: { "project_id": "proj-uuid", "type": "send_email", "payload": {"to": "x@y.com"} }
         Header: Authorization: Bearer <api_key>
```

1. **`routes.go`** — chi router matches `POST /api/v1/job/submit` → middleware chain → `SubmitHandler`
2. **`middleware/auth.go` `APIKeyAuth`** — reads `Authorization` header, looks up user in Postgres, stores user in request context
3. **`middleware/ratelimit.go` `RateLimit`** — checks token bucket for this API key
4. **`api/job.go` `SubmitHandler`** — reads user from context, decodes body
5. **`jobs/validator.go` `ValidateSubmit`** — checks project_id, type, payload
6. **`api/project.go` `loadOwnedProject`** — verifies project belongs to user
7. **`api/job.go`** — creates `models.Job` in Postgres, calls `heuristics.GetQueue("send_email")` → `"queue:high"`
8. **`redis.LPush("queue:high", job.ID)`** — job ID is now at the head of the Redis list
9. Response: `202 Accepted` with `{"job_id": "..."}`

### Phase 2: Worker picks up

1. **`workers/pool.go`** — `highPriorityPool` has workers listening on `"queue:high"`
2. **`workers/worker.go` `Loop`** — `BRPop("queue:high", 5s)` — returns `["queue:high", "<job-id>"]`
3. **`processJob`** — opens DB transaction, `SELECT ... FOR UPDATE WHERE id = <job-id>`
4. Checks `status == "queued"`, sets `status = "running"`, commits
5. **`executeTask`** — calls `tasks.Get("send_email")` → returns `&MockEmailSender{}`
6. **`tasks/processor.go` `Process`** — prints simulation, returns nil or error

### Phase 3a: Success

1. `processJob` updates Postgres: `status = "completed"`, `duration = Nms`
2. `metrics.JobsProcessedTotal.WithLabelValues("queue:high", "completed").Inc()`

### Phase 3b: Failure (with retries remaining)

1. `handleFailure` — `job.RetryCount++` (e.g. 1), `job.Status = "queued"`, saves to Postgres
2. `rdb.LPush("queue:high", job.ID)` — re-queued on the same queue
3. Next iteration of worker loop picks it up again

### Phase 3c: Permanent failure (retries exhausted)

1. `job.RetryCount > job.MaxRetries` — `job.Status = "failed"`, saved to Postgres
2. `rdb.LPush("queue:dlq", <full job JSON>)` — DLQ push
3. `go ai.HandleDLQWithAI(...)` — AI summary stored in Redis at `dlq_summary:<job-id>`

### Phase 4: Query status

```
Client → GET /api/v1/job/status/<job-id>
```

1. `StatusHandler` — loads job from Postgres, verifies project ownership
2. Returns job JSON with current `status`, `retry_count`, `duration`

### Phase 5: Reaper (crash recovery)

1. **`workers/reaper.go`** — every 5 minutes, queries `status='running' AND updated_at < 1h ago`
2. For each stuck job: `status = "queued"`, `LPush(GetQueue(job.Type), job.ID)`
3. Job re-enters the queue and is processed again

### Phase 6: Autoscaler (load response)

1. **`workers/autoscaler.go`** — every 5s, `LLen("queue:high")` → current depth
2. If depth > 20: `pool.ScaleUp(2)` → 2 new worker goroutines start, each running `worker.Loop`
3. If depth < 5: `pool.ScaleDown(1)` → 1 worker's context cancelled → goroutine exits after current job

---

## Appendix: Go patterns used in this project

| Pattern | Where used | What it is |
|---------|------------|------------|
| Constructor function `NewXxx` | All structs | Go equivalent of classes with init |
| Method on struct `(r *T) Method()` | All handlers, workers | Behaviour attached to data |
| Interface as dependency | `tasks.Processor` | Swap implementations without changing callers |
| Sentinel errors | `jobs/validator.go` | Named error values for `errors.Is` |
| Context propagation | Everywhere | Cancellation and deadlines flow through call chains |
| Goroutines + WaitGroup | `workers/pool.go` | Concurrency with clean shutdown |
| Mutex | `workers/pool.go` | Protect shared state from data races |
| Defer for cleanup | Transaction rollback, mutex unlock, ticker stop | Execute cleanup regardless of return path |
| `select` on channels | Worker loop, autoscaler | Non-blocking multi-channel waits |
| Middleware chain | `routes.go` | Composable request pre/post-processing |
| Struct tags | `models`, request types | Metadata for GORM and JSON |
| Short `if err :=` | Error checks | Scope-limited error variable |
| `map[string]struct{}` | `AllowedJobTypes` | Memory-efficient set |
| Closure capturing | `ScaleUp` goroutine | Function retains access to outer variables |
| Type assertion `.(T)` | `middleware/auth.go` | Extract concrete type from interface |
| `fmt.Errorf("%w", err)` | Task processors | Wrap errors for chain inspection |
| `errors.Is` | Auth, worker | Check error identity through wrap chains |

---

*This document covers every file in `jobqueue/` as of Phase 1 completion. Add sections for new files (`jobs/dlq.go`, `jobs/results.go`, UI components) when they are implemented.*
