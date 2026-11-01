"use client";

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { MetricsPanel } from "@/components/metrics-panel";
import { SiteNav } from "@/components/site-nav";
import * as api from "@/lib/api";
import {
  JOB_TYPES,
  defaultPayloads,
  type FailureIncident,
  type Job,
  type JobEvent,
  type JobType,
  type Project,
} from "@/lib/types";
import { cn } from "@/lib/utils";

const KEY_STORAGE = "jobqueue.apiKey";
const EMAIL_STORAGE = "jobqueue.email";
const PROJECT_STORAGE = "jobqueue.projectId";

type Section = "overview" | "metrics" | "projects" | "enqueue" | "jobs" | "failures" | "account";

const NAV: { id: Section; label: string }[] = [
  { id: "overview", label: "Overview" },
  { id: "metrics", label: "Metrics" },
  { id: "projects", label: "Projects" },
  { id: "enqueue", label: "Enqueue" },
  { id: "jobs", label: "Jobs" },
  { id: "failures", label: "Failures" },
  { id: "account", label: "Account" },
];

function statusClass(status: string) {
  switch (status) {
    case "completed":
      return "bg-emerald-500/15 text-emerald-300";
    case "failed":
      return "bg-red-500/15 text-red-300";
    case "running":
    case "claimed":
      return "bg-sky-500/15 text-sky-300";
    case "discarded":
      return "bg-neutral-500/20 text-neutral-400";
    default:
      return "bg-amber-500/15 text-amber-200";
  }
}

function when(value?: string) {
  if (!value || value.startsWith("0001")) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function short(id: string) {
  return id.length > 12 ? `${id.slice(0, 8)}…${id.slice(-4)}` : id;
}

export function ConsoleApp() {
  const [section, setSection] = useState<Section>("overview");
  const [ready, setReady] = useState(false);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [showKey, setShowKey] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [apiUp, setApiUp] = useState<"checking" | "online" | "offline">("checking");

  const [projects, setProjects] = useState<Project[]>([]);
  const [projectName, setProjectName] = useState("demo");
  const [projectId, setProjectId] = useState("");

  const [jobs, setJobs] = useState<Job[]>([]);
  const [jobType, setJobType] = useState<JobType>("echo");
  const [payloadText, setPayloadText] = useState(defaultPayloads.echo);
  const [idempotencyKey, setIdempotencyKey] = useState("");
  const [customJobId, setCustomJobId] = useState("");

  const [selectedJobId, setSelectedJobId] = useState("");
  const [detailJob, setDetailJob] = useState<Job | null>(null);
  const [events, setEvents] = useState<JobEvent[]>([]);

  const [incidents, setIncidents] = useState<FailureIncident[]>([]);
  const [incident, setIncident] = useState<FailureIncident | null>(null);
  const [requeuePayload, setRequeuePayload] = useState("{}");

  const flash = (message: string) => {
    setNotice(message);
    setError("");
  };
  const fail = (err: unknown) => {
    setError(err instanceof Error ? err.message : String(err));
    setNotice("");
  };
  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    try {
      await fn();
    } catch (err) {
      fail(err);
    } finally {
      setBusy(false);
    }
  };

  const go = (id: Section) => {
    setSection(id);
    window.history.replaceState(null, "", `#${id}`);
  };

  const chooseProject = (id: string) => {
    setProjectId(id);
    localStorage.setItem(PROJECT_STORAGE, id);
    setIncident(null);
    setDetailJob(null);
    setSelectedJobId("");
  };

  const refreshProjects = useCallback(async (key: string) => {
    const list = (await api.listProjects(key)) ?? [];
    setProjects(list);
    setProjectId((current) => {
      if (current && list.some((p) => p.ID === current)) return current;
      const stored = localStorage.getItem(PROJECT_STORAGE) ?? "";
      const next = list.find((p) => p.ID === stored)?.ID ?? list[0]?.ID ?? "";
      if (next) localStorage.setItem(PROJECT_STORAGE, next);
      return next;
    });
  }, []);

  const refreshJobs = useCallback(async (key: string, project: string) => {
    setJobs((await api.listJobs(key, project)) ?? []);
  }, []);

  const refreshFailures = useCallback(async (key: string, project: string) => {
    setIncidents((await api.listFailures(key, project)) ?? []);
  }, []);

  const openJob = useCallback(async (key: string, jobID: string) => {
    const status = await api.getJobStatus(key, jobID);
    setSelectedJobId(jobID);
    setDetailJob(status.job);
    setEvents(status.events ?? []);
  }, []);

  useEffect(() => {
    const storedKey = localStorage.getItem(KEY_STORAGE) ?? "";
    setApiKey(storedKey);
    setEmail(localStorage.getItem(EMAIL_STORAGE) ?? "");
    setProjectId(localStorage.getItem(PROJECT_STORAGE) ?? "");
    setIdempotencyKey(crypto.randomUUID());
    const hash = window.location.hash.replace("#", "");
    if (NAV.some((item) => item.id === hash)) setSection(hash as Section);
    setReady(true);
  }, []);

  useEffect(() => {
    if (apiKey) localStorage.setItem(KEY_STORAGE, apiKey);
  }, [apiKey]);

  useEffect(() => {
    let stop = false;
    const ping = async () => {
      try {
        const res = await fetch("/api/v1/projects");
        if (!stop) setApiUp(res.ok || res.status === 401 ? "online" : "offline");
      } catch {
        if (!stop) setApiUp("offline");
      }
    };
    ping();
    const timer = setInterval(ping, 8000);
    return () => {
      stop = true;
      clearInterval(timer);
    };
  }, []);

  useEffect(() => {
    if (!ready || !apiKey) return;
    refreshProjects(apiKey).catch(fail);
  }, [ready, apiKey, refreshProjects]);

  useEffect(() => {
    if (!apiKey || !projectId) return;
    refreshJobs(apiKey, projectId).catch(() => undefined);
    refreshFailures(apiKey, projectId).catch(() => undefined);
    const timer = setInterval(() => {
      refreshJobs(apiKey, projectId).catch(() => undefined);
      refreshFailures(apiKey, projectId).catch(() => undefined);
      if (selectedJobId) openJob(apiKey, selectedJobId).catch(() => undefined);
    }, 3000);
    return () => clearInterval(timer);
  }, [apiKey, projectId, refreshJobs, refreshFailures, selectedJobId, openJob]);

  const project = projects.find((item) => item.ID === projectId);
  const counts = useMemo(() => {
    const by: Record<string, number> = {};
    for (const job of jobs) by[job.Status] = (by[job.Status] ?? 0) + 1;
    return by;
  }, [jobs]);

  const signOut = () => {
    localStorage.removeItem(KEY_STORAGE);
    setApiKey("");
    setProjects([]);
    setJobs([]);
    setIncidents([]);
    setDetailJob(null);
    flash("Signed out. The API key was removed from this browser.");
    go("account");
  };

  return (
    <div className="min-h-screen bg-[#07080a] text-neutral-100">
      <SiteNav active="console" />
      <div className="mx-auto grid max-w-6xl gap-6 px-5 py-6 md:grid-cols-[180px_1fr]">
        <aside className="flex gap-1 overflow-x-auto md:flex-col">
          {NAV.map((item) => (
            <button
              key={item.id}
              type="button"
              onClick={() => go(item.id)}
              className={cn(
                "shrink-0 rounded-lg px-3 py-2 text-left text-sm",
                section === item.id
                  ? "bg-white text-black"
                  : "text-neutral-400 hover:bg-white/5 hover:text-white",
              )}
            >
              {item.label}
            </button>
          ))}
        </aside>

        <div className="min-w-0 space-y-4">
          <div className="flex flex-wrap items-center gap-3">
            <span className="inline-flex items-center gap-2 text-xs text-neutral-400">
              <span
                className={cn(
                  "h-2 w-2 rounded-full",
                  apiUp === "online" && "bg-emerald-400",
                  apiUp === "offline" && "bg-red-400",
                  apiUp === "checking" && "bg-amber-300",
                )}
              />
              {apiUp === "online" ? "API online" : apiUp === "offline" ? "API offline" : "Checking API"}
            </span>
            <label className="text-xs text-neutral-500">
              Project
              <select
                className="ml-2 h-8 rounded-md border border-white/10 bg-black px-2 text-sm text-white"
                value={projectId}
                onChange={(event) => chooseProject(event.target.value)}
                disabled={!projects.length}
              >
                {!projects.length ? <option value="">No projects</option> : null}
                {projects.map((item) => (
                  <option key={item.ID} value={item.ID}>
                    {item.Name}
                  </option>
                ))}
              </select>
            </label>
            <button
              type="button"
              className="ml-auto text-xs text-neutral-400 hover:text-white"
              onClick={() =>
                run(async () => {
                  if (!apiKey) return;
                  await refreshProjects(apiKey);
                  if (projectId) {
                    await refreshJobs(apiKey, projectId);
                    await refreshFailures(apiKey, projectId);
                  }
                  flash("Refreshed");
                })
              }
            >
              Refresh
            </button>
          </div>

          {notice ? (
            <p className="rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-200">
              {notice}
            </p>
          ) : null}
          {error ? (
            <p className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-200">
              {error}
            </p>
          ) : null}
          {apiUp === "offline" ? (
            <p className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-100">
              The Go API is not reachable at 127.0.0.1:8080. From <code>jobqueue/</code>, start
              Postgres and Redis, then run <code>go run ./cmd/server</code>.
            </p>
          ) : null}

          {section === "overview" ? (
            <section className="space-y-4">
              <header>
                <h1 className="text-2xl font-semibold text-white">
                  {project ? project.Name : "No project selected"}
                </h1>
                <p className="mt-1 text-sm text-neutral-400">
                  {jobs.length} jobs · {incidents.length} failure incidents
                </p>
              </header>
              <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
                {["queued", "running", "completed", "failed"].map((status) => (
                  <div key={status} className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
                    <p className="text-xs tracking-wide text-neutral-500 uppercase">{status}</p>
                    <p className="mt-2 text-2xl font-semibold text-white">{counts[status] ?? 0}</p>
                  </div>
                ))}
              </div>
              <JobTable
                jobs={jobs.slice(0, 8)}
                onOpen={(id) =>
                  run(async () => {
                    await openJob(apiKey, id);
                    go("jobs");
                  })
                }
              />
            </section>
          ) : null}

          {section === "metrics" ? <MetricsPanel jobs={jobs} /> : null}

          {section === "projects" ? (
            <Panel title="Projects" hint="Jobs are scoped to a project.">
              <div className="flex gap-2">
                <input
                  className={fieldClass}
                  value={projectName}
                  onChange={(event) => setProjectName(event.target.value)}
                  placeholder="Project name"
                />
                <button
                  type="button"
                  className={primaryClass}
                  disabled={busy || !apiKey}
                  onClick={() =>
                    run(async () => {
                      const created = await api.createProject(apiKey, projectName);
                      chooseProject(created.ID);
                      await refreshProjects(apiKey);
                      flash(`Created ${created.Name}`);
                    })
                  }
                >
                  Create
                </button>
              </div>
              <ul className="mt-4 divide-y divide-white/10">
                {projects.map((item) => (
                  <li key={item.ID} className="flex items-center justify-between gap-3 py-3 text-sm">
                    <div>
                      <p className="text-white">{item.Name}</p>
                      <p className="font-mono text-xs text-neutral-500">{item.ID}</p>
                    </div>
                    <button
                      type="button"
                      className={item.ID === projectId ? primaryClass : ghostClass}
                      onClick={() => {
                        chooseProject(item.ID);
                        flash(`Using ${item.Name}`);
                      }}
                    >
                      {item.ID === projectId ? "Selected" : "Use"}
                    </button>
                  </li>
                ))}
                {!projects.length ? (
                  <li className="py-6 text-sm text-neutral-500">No projects yet.</li>
                ) : null}
              </ul>
            </Panel>
          ) : null}

          {section === "enqueue" ? (
            <Panel title="Enqueue a job" hint="Requires a project and an Idempotency-Key.">
              <label className="text-xs text-neutral-500">
                Type
                <select
                  className={`${fieldClass} mt-1`}
                  value={jobType}
                  onChange={(event) => {
                    const next = event.target.value as JobType;
                    setJobType(next);
                    setPayloadText(defaultPayloads[next]);
                  }}
                >
                  {JOB_TYPES.map((type) => (
                    <option key={type} value={type}>
                      {type}
                    </option>
                  ))}
                </select>
              </label>
              <label className="mt-3 block text-xs text-neutral-500">
                Payload
                <textarea
                  className={`${fieldClass} mt-1 min-h-36 font-mono`}
                  value={payloadText}
                  onChange={(event) => setPayloadText(event.target.value)}
                />
              </label>
              <label className="mt-3 block text-xs text-neutral-500">
                Idempotency-Key
                <div className="mt-1 flex gap-2">
                  <input
                    className={fieldClass}
                    value={idempotencyKey}
                    onChange={(event) => setIdempotencyKey(event.target.value)}
                  />
                  <button
                    type="button"
                    className={ghostClass}
                    onClick={() => setIdempotencyKey(crypto.randomUUID())}
                  >
                    New
                  </button>
                </div>
              </label>
              <label className="mt-3 block text-xs text-neutral-500">
                Optional job id
                <input
                  className={`${fieldClass} mt-1`}
                  value={customJobId}
                  onChange={(event) => setCustomJobId(event.target.value)}
                  placeholder="Leave empty to generate one"
                />
              </label>
              <button
                type="button"
                className={`${primaryClass} mt-4`}
                disabled={busy || !apiKey || !projectId || !idempotencyKey}
                onClick={() =>
                  run(async () => {
                    const payload = JSON.parse(payloadText) as Record<string, unknown>;
                    const result = await api.submitJob(
                      apiKey,
                      {
                        project_id: projectId,
                        type: jobType,
                        payload,
                        job_id: customJobId || undefined,
                      },
                      idempotencyKey,
                    );
                    await refreshJobs(apiKey, projectId);
                    await openJob(apiKey, result.job_id);
                    flash(
                      result.idempotent_replay
                        ? `Replayed existing job ${result.job_id}`
                        : `Accepted ${result.job_id}`,
                    );
                    go("jobs");
                  })
                }
              >
                Submit
              </button>
              {!apiKey ? <p className="mt-3 text-sm text-neutral-500">Sign in on Account first.</p> : null}
              {apiKey && !projectId ? (
                <p className="mt-3 text-sm text-neutral-500">Create a project before submitting.</p>
              ) : null}
            </Panel>
          ) : null}

          {section === "jobs" ? (
            <section className="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
              <Panel title="Jobs" hint={project ? project.Name : "Select a project"}>
                <JobTable
                  jobs={jobs}
                  activeId={selectedJobId}
                  onOpen={(id) => run(async () => openJob(apiKey, id))}
                />
              </Panel>
              <Panel title="Timeline" hint={detailJob ? short(detailJob.ID) : "Select a job"}>
                {detailJob ? (
                  <div className="space-y-3 text-sm">
                    <p>
                      <span className={cn("rounded-full px-2 py-0.5 text-xs", statusClass(detailJob.Status))}>
                        {detailJob.Status}
                      </span>
                      <span className="ml-2 text-neutral-400">
                        {detailJob.RetryCount}/{detailJob.MaxRetries} retries
                      </span>
                    </p>
                    <p className="break-all font-mono text-xs text-neutral-400">{detailJob.ID}</p>
                    <p className="text-neutral-400">Key {detailJob.IdempotencyKey || "—"}</p>
                    {detailJob.LastError ? <p className="text-red-300">{detailJob.LastError}</p> : null}
                    <pre className="overflow-auto rounded-lg bg-black/50 p-3 text-xs text-neutral-300">
                      {pretty(detailJob.Payload)}
                    </pre>
                    <ol className="space-y-2 border-l border-white/10 pl-3">
                      {events.map((event) => (
                        <li key={event.ID}>
                          <p className="text-white">{event.Type}</p>
                          <p className="text-xs text-neutral-500">{when(event.CreatedAt)}</p>
                        </li>
                      ))}
                    </ol>
                  </div>
                ) : (
                  <p className="text-sm text-neutral-500">Open a job to see its events.</p>
                )}
              </Panel>
            </section>
          ) : null}

          {section === "failures" ? (
            <section className="grid gap-4 lg:grid-cols-2">
              <Panel title="Incidents" hint="Grouped by fingerprint">
                <ul className="space-y-2">
                  {incidents.map((item) => (
                    <li key={item.Fingerprint}>
                      <button
                        type="button"
                        className="w-full rounded-lg border border-white/10 px-3 py-3 text-left hover:bg-white/5"
                        onClick={() =>
                          run(async () => {
                            const detail = await api.getFailure(apiKey, item.Fingerprint);
                            setIncident(detail);
                            setRequeuePayload("{}");
                          })
                        }
                      >
                        <p className="text-sm text-white">
                          {item.JobType} · {item.ErrorClass}
                        </p>
                        <p className="mt-1 text-xs text-neutral-500">
                          {item.OccurrenceCount} occurrences · {when(item.LastSeenAt)}
                        </p>
                      </button>
                    </li>
                  ))}
                  {!incidents.length ? (
                    <li className="text-sm text-neutral-500">
                      No incidents. Enqueue <code>always_fail</code> and wait for retries to exhaust.
                    </li>
                  ) : null}
                </ul>
              </Panel>
              <Panel title="Incident" hint="Requeue sends the job back to the stream">
                {incident ? (
                  <div className="space-y-3 text-sm">
                    <p className="text-neutral-300">{incident.Diagnosis || "No diagnosis stored yet."}</p>
                    <p className="text-xs text-neutral-500">Payload keys {incident.PayloadKeys || "—"}</p>
                    <textarea
                      className={`${fieldClass} min-h-24 font-mono`}
                      value={requeuePayload}
                      onChange={(event) => setRequeuePayload(event.target.value)}
                    />
                    <ul className="space-y-2">
                      {(incident.recent_job_ids ?? []).map((id) => (
                        <li key={id} className="flex flex-wrap items-center gap-2">
                          <span className="font-mono text-xs text-neutral-400">{short(id)}</span>
                          <button
                            type="button"
                            className={ghostClass}
                            onClick={() =>
                              run(async () => {
                                const failed = await api.getFailedJob(apiKey, id);
                                setDetailJob(failed.job);
                                setEvents(failed.events ?? []);
                                setSelectedJobId(id);
                                setRequeuePayload(pretty(failed.job.Payload));
                                flash("Loaded failed job");
                              })
                            }
                          >
                            Inspect
                          </button>
                          <button
                            type="button"
                            className={primaryClass}
                            disabled={busy}
                            onClick={() =>
                              run(async () => {
                                const parsed = JSON.parse(requeuePayload) as Record<string, unknown>;
                                const payload =
                                  parsed && Object.keys(parsed).length > 0 ? parsed : undefined;
                                const result = await api.requeueFailedJob(apiKey, id, payload);
                                flash(`Requeued ${result.job_id}`);
                                await refreshJobs(apiKey, projectId);
                                await refreshFailures(apiKey, projectId);
                              })
                            }
                          >
                            Requeue
                          </button>
                          <button
                            type="button"
                            className="h-8 rounded-md bg-red-500/20 px-3 text-xs text-red-200"
                            disabled={busy}
                            onClick={() =>
                              run(async () => {
                                await api.discardFailedJob(apiKey, id);
                                flash(`Discarded ${short(id)}`);
                                await refreshJobs(apiKey, projectId);
                                await refreshFailures(apiKey, projectId);
                              })
                            }
                          >
                            Discard
                          </button>
                        </li>
                      ))}
                    </ul>
                  </div>
                ) : (
                  <p className="text-sm text-neutral-500">Select an incident.</p>
                )}
              </Panel>
            </section>
          ) : null}

          {section === "account" ? (
            <Panel title="Account" hint="Register once, then log in. The key stays in this browser.">
              <label className="block text-xs text-neutral-500">
                Email
                <input
                  className={`${fieldClass} mt-1`}
                  value={email}
                  autoComplete="email"
                  onChange={(event) => {
                    setEmail(event.target.value);
                    localStorage.setItem(EMAIL_STORAGE, event.target.value);
                  }}
                />
              </label>
              <label className="mt-3 block text-xs text-neutral-500">
                Password
                <input
                  className={`${fieldClass} mt-1`}
                  type="password"
                  value={password}
                  autoComplete="current-password"
                  onChange={(event) => setPassword(event.target.value)}
                />
              </label>
              <div className="mt-4 flex gap-2">
                <button
                  type="button"
                  className={primaryClass}
                  disabled={busy || !email || !password}
                  onClick={() =>
                    run(async () => {
                      const result = await api.register(email, password);
                      setApiKey(result.api_key);
                      flash("Registered. API key saved.");
                      go("projects");
                    })
                  }
                >
                  Register
                </button>
                <button
                  type="button"
                  className={ghostClass}
                  disabled={busy || !email || !password}
                  onClick={() =>
                    run(async () => {
                      const result = await api.login(email, password);
                      setApiKey(result.api_key);
                      flash("Logged in.");
                      go("overview");
                    })
                  }
                >
                  Log in
                </button>
                {apiKey ? (
                  <button type="button" className={ghostClass} onClick={signOut}>
                    Sign out
                  </button>
                ) : null}
              </div>
              <label className="mt-5 block text-xs text-neutral-500">
                API key
                <div className="mt-1 flex gap-2">
                  <input
                    className={fieldClass}
                    type={showKey ? "text" : "password"}
                    value={apiKey}
                    onChange={(event) => setApiKey(event.target.value.trim())}
                  />
                  <button type="button" className={ghostClass} onClick={() => setShowKey((value) => !value)}>
                    {showKey ? "Hide" : "Show"}
                  </button>
                </div>
              </label>
            </Panel>
          ) : null}
        </div>
      </div>
    </div>
  );
}

const fieldClass =
  "h-10 w-full rounded-lg border border-white/10 bg-black/40 px-3 text-sm text-white outline-none placeholder:text-neutral-600 focus:border-white/30";
const primaryClass =
  "inline-flex h-9 items-center rounded-lg bg-white px-3 text-sm font-medium text-black disabled:opacity-40";
const ghostClass =
  "inline-flex h-9 items-center rounded-lg border border-white/15 px-3 text-sm text-white disabled:opacity-40";

function Panel({
  title,
  hint,
  children,
}: {
  title: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <section className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
      <h2 className="text-lg font-medium text-white">{title}</h2>
      {hint ? <p className="mt-1 mb-4 text-sm text-neutral-500">{hint}</p> : <div className="mb-4" />}
      {children}
    </section>
  );
}

function JobTable({
  jobs,
  activeId,
  onOpen,
}: {
  jobs: Job[];
  activeId?: string;
  onOpen: (id: string) => void;
}) {
  if (!jobs.length) {
    return <p className="text-sm text-neutral-500">No jobs in this project.</p>;
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead className="text-xs tracking-wide text-neutral-500 uppercase">
          <tr>
            <th className="py-2 font-medium">Type</th>
            <th className="font-medium">Status</th>
            <th className="font-medium">Updated</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {jobs.map((job) => (
            <tr key={job.ID} className={cn("border-t border-white/10", job.ID === activeId && "bg-white/5")}>
              <td className="py-3">
                <p className="text-white">{job.Type}</p>
                <p className="font-mono text-xs text-neutral-500">{short(job.ID)}</p>
              </td>
              <td>
                <span className={cn("rounded-full px-2 py-0.5 text-xs", statusClass(job.Status))}>
                  {job.Status}
                </span>
              </td>
              <td className="text-xs text-neutral-500">{when(job.UpdatedAt)}</td>
              <td className="text-right">
                <button type="button" className={ghostClass} onClick={() => onOpen(job.ID)}>
                  Open
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function pretty(raw: string) {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}
