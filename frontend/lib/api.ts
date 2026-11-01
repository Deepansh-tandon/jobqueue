import type {
  FailureIncident,
  FailureEvent,
  Job,
  JobEvent,
  Project,
  SubmitResponse,
} from "./types";

const BASE = "/api/v1";

async function readError(res: Response): Promise<string> {
  const text = (await res.text()).replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim();
  const msg = text || res.statusText;
  return msg.length > 240 ? `${msg.slice(0, 240)}…` : msg;
}

async function request<T>(
  path: string,
  init: RequestInit & { apiKey?: string } = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  if (!headers.has("Content-Type") && init.body) {
    headers.set("Content-Type", "application/json");
  }
  const { apiKey, ...rest } = init;
  if (apiKey) {
    headers.set("Authorization", `Bearer ${apiKey}`);
  }
  let res: Response;
  try {
    res = await fetch(`${BASE}${path}`, { ...rest, headers });
  } catch {
    throw new Error(
      "Cannot reach the API. From jobqueue/, run: go run ./cmd/server (needs Postgres + Redis on .env ports).",
    );
  }
  if (!res.ok) {
    throw new Error(await readError(res));
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

export function register(email: string, password: string) {
  return request<{ api_key: string }>("/register", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
}

export function login(email: string, password: string) {
  return request<{ api_key: string }>("/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
}

export function listProjects(apiKey: string) {
  return request<Project[]>("/projects", { apiKey });
}

export function createProject(apiKey: string, name: string) {
  return request<Project>("/projects", {
    method: "POST",
    apiKey,
    body: JSON.stringify({ name }),
  });
}

export function listJobs(apiKey: string, projectID: string) {
  return request<Job[]>(`/job/list?projectID=${encodeURIComponent(projectID)}`, {
    apiKey,
  });
}

export function getJobStatus(apiKey: string, jobID: string) {
  return request<{ job: Job; events: JobEvent[] }>(
    `/job/status/${encodeURIComponent(jobID)}`,
    { apiKey },
  );
}

export function submitJob(
  apiKey: string,
  body: {
    project_id: string;
    type: string;
    payload: Record<string, unknown>;
    job_id?: string;
  },
  idempotencyKey: string,
) {
  return request<SubmitResponse>("/job/submit", {
    method: "POST",
    apiKey,
    headers: { "Idempotency-Key": idempotencyKey },
    body: JSON.stringify(body),
  });
}

export function listFailures(apiKey: string, projectID: string) {
  return request<FailureIncident[]>(
    `/failures?projectID=${encodeURIComponent(projectID)}`,
    { apiKey },
  );
}

export function getFailure(apiKey: string, fingerprint: string) {
  return request<FailureIncident>(
    `/failures/${encodeURIComponent(fingerprint)}`,
    { apiKey },
  );
}

export function getFailedJob(apiKey: string, jobID: string) {
  return request<{
    job: Job;
    events: JobEvent[];
    failure: FailureEvent;
    incident: FailureIncident;
  }>(`/failures/jobs/${encodeURIComponent(jobID)}`, { apiKey });
}

export function requeueFailedJob(
  apiKey: string,
  jobID: string,
  payload?: Record<string, unknown>,
) {
  return request<{ job_id: string; status: string }>(
    `/failures/jobs/${encodeURIComponent(jobID)}/requeue`,
    {
      method: "POST",
      apiKey,
      body: JSON.stringify(payload ? { payload } : {}),
    },
  );
}

export function discardFailedJob(apiKey: string, jobID: string) {
  return request<void>(`/failures/jobs/${encodeURIComponent(jobID)}`, {
    method: "DELETE",
    apiKey,
  });
}
