export const JOB_TYPES = [
  "echo",
  "sleep",
  "fail_until_n",
  "always_fail",
  "send_email",
  "generate_receipt",
  "summarize_text",
] as const;

export type JobType = (typeof JOB_TYPES)[number];

export type Project = {
  ID: string;
  Name: string;
  UserID: string;
  CreatedAt: string;
};

export type Job = {
  ID: string;
  Type: string;
  Payload: string;
  Status: string;
  ExecuteAt?: string;
  Duration: number;
  ProjectID: string;
  IdempotencyKey: string;
  RequestHash: string;
  MaxRetries: number;
  RetryCount: number;
  LastError: string;
  CreatedAt: string;
  UpdatedAt: string;
};

export type JobEvent = {
  ID: string;
  JobID: string;
  Type: string;
  Payload: string;
  CreatedAt: string;
};

export type FailureIncident = {
  Fingerprint: string;
  JobType: string;
  ErrorClass: string;
  PayloadKeys: string;
  Diagnosis: string;
  OccurrenceCount: number;
  FirstSeenAt: string;
  LastSeenAt: string;
  recent_job_ids?: string[];
};

export type FailureEvent = {
  ID: string;
  JobID: string;
  Fingerprint: string;
  ErrorRaw: string;
  RetryCount: number;
  DurationMs: number;
  CreatedAt: string;
};

export type SubmitResponse = {
  job_id: string;
  idempotent_replay: boolean;
};

export const defaultPayloads: Record<JobType, string> = {
  echo: JSON.stringify({ msg: "hello from the console" }, null, 2),
  sleep: JSON.stringify({ ms: 2000 }, null, 2),
  fail_until_n: JSON.stringify({ fail_times: 2 }, null, 2),
  always_fail: JSON.stringify({ k: true }, null, 2),
  send_email: JSON.stringify(
    { to: "ops@example.com", subject: "queue ping" },
    null,
    2,
  ),
  generate_receipt: JSON.stringify(
    { to: "Ada", item: "Widgets", amount: 12.5, is_paid: true },
    null,
    2,
  ),
  summarize_text: JSON.stringify(
    { text: "Workers lease jobs from Redis streams and persist status in Postgres." },
    null,
    2,
  ),
};
