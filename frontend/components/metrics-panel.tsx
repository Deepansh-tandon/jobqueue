"use client";

import { useEffect, useState } from "react";
import type { Job } from "@/lib/types";

type Point = { t: number; v: number };
type Series = { label: string; points: Point[] };

const COLORS = ["#e7e5e4", "#38bdf8", "#fbbf24", "#f87171", "#34d399", "#c084fc"];

const QUERIES: { title: string; expr: string; hint: string }[] = [
  {
    title: "Active workers",
    expr: "jobqueue_workers_active",
    hint: "Workers currently running, per stream.",
  },
  {
    title: "Stream depth",
    expr: "jobqueue_queue_length",
    hint: "Messages waiting in Redis streams.",
  },
  {
    title: "Jobs processed",
    expr: "sum by (status) (jobqueue_jobs_processed_total)",
    hint: "Cumulative completions and permanent failures.",
  },
  {
    title: "Failure rate",
    expr: "sum by (type) (rate(jobqueue_job_failures_total[1m]))",
    hint: "Retries and dead-letter events per second.",
  },
];

async function queryRange(expr: string): Promise<Series[]> {
  const end = Math.floor(Date.now() / 1000);
  const start = end - 30 * 60;
  const url = `/prom/api/v1/query_range?query=${encodeURIComponent(expr)}&start=${start}&end=${end}&step=15`;
  const res = await fetch(url);
  if (!res.ok) throw new Error("Prometheus is not reachable on port 9090");
  const body = (await res.json()) as {
    status: string;
    error?: string;
    data?: { result?: { metric: Record<string, string>; values: [number, string][] }[] };
  };
  if (body.status !== "success") throw new Error(body.error || "Prometheus query failed");
  return (body.data?.result ?? []).map((row) => {
    const label =
      Object.entries(row.metric)
        .filter(([key]) => key !== "__name__" && key !== "job" && key !== "instance")
        .map(([key, value]) => `${key}=${value}`)
        .join(" ") || "total";
    return {
      label,
      points: row.values.map(([t, v]) => ({ t, v: Number(v) })),
    };
  });
}

export function MetricsPanel({ jobs }: { jobs: Job[] }) {
  const [series, setSeries] = useState<Record<string, Series[]>>({});
  const [promError, setPromError] = useState("");

  useEffect(() => {
    let stop = false;
    const load = async () => {
      try {
        const entries = await Promise.all(
          QUERIES.map(async (query) => [query.expr, await queryRange(query.expr)] as const),
        );
        if (stop) return;
        setSeries(Object.fromEntries(entries));
        setPromError("");
      } catch (err) {
        if (!stop) setPromError(err instanceof Error ? err.message : String(err));
      }
    };
    load();
    const timer = setInterval(load, 10000);
    return () => {
      stop = true;
      clearInterval(timer);
    };
  }, []);

  const byStatus = count(jobs, (job) => job.Status);
  const byType = count(jobs, (job) => job.Type);

  return (
    <div className="space-y-4">
      <section className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
        <h2 className="text-lg font-medium text-white">This project</h2>
        <p className="mt-1 mb-4 text-sm text-neutral-500">
          Counts from the jobs you can list. These are scoped to the selected project.
        </p>
        <div className="grid gap-6 md:grid-cols-2">
          <Bars title="By status" rows={byStatus} />
          <Bars title="By type" rows={byType} />
        </div>
      </section>

      <section className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 className="text-lg font-medium text-white">Cluster</h2>
            <p className="mt-1 max-w-xl text-sm text-neutral-500">
              Prometheus scrapes the Go process at <code>/metrics</code>. These series are for the
              whole server, not one user. Grafana shows the same data at{" "}
              <a className="text-white underline" href="http://localhost:3002/d/jobqueue/jobqueue" target="_blank" rel="noreferrer">
                localhost:3002
              </a>{" "}
              (admin / admin).
            </p>
          </div>
        </div>
        {promError ? (
          <p className="mt-4 rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-100">
            {promError}. Start it with <code>docker compose up -d prometheus grafana</code> from{" "}
            <code>jobqueue/</code>.
          </p>
        ) : null}
        <div className="mt-4 grid gap-4 lg:grid-cols-2">
          {QUERIES.map((query) => (
            <div key={query.expr} className="rounded-xl border border-white/10 p-3">
              <p className="text-sm text-white">{query.title}</p>
              <p className="mb-2 text-xs text-neutral-500">{query.hint}</p>
              <LineChart series={series[query.expr] ?? []} />
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

function count(jobs: Job[], key: (job: Job) => string) {
  const map = new Map<string, number>();
  for (const job of jobs) map.set(key(job), (map.get(key(job)) ?? 0) + 1);
  return [...map.entries()].sort((a, b) => b[1] - a[1]);
}

function Bars({ title, rows }: { title: string; rows: [string, number][] }) {
  const max = Math.max(1, ...rows.map(([, n]) => n));
  return (
    <div>
      <p className="mb-2 text-sm text-neutral-300">{title}</p>
      {!rows.length ? <p className="text-sm text-neutral-500">No jobs yet.</p> : null}
      <ul className="space-y-2">
        {rows.map(([label, n], index) => (
          <li key={label}>
            <div className="mb-1 flex justify-between text-xs text-neutral-400">
              <span>{label}</span>
              <span>{n}</span>
            </div>
            <div className="h-2 rounded-full bg-white/10">
              <div
                className="h-2 rounded-full"
                style={{
                  width: `${(n / max) * 100}%`,
                  background: COLORS[index % COLORS.length],
                }}
              />
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}

function LineChart({ series }: { series: Series[] }) {
  const width = 420;
  const height = 140;
  const pad = 8;
  const points = series.flatMap((item) => item.points);
  if (!points.length) {
    return <p className="flex h-[140px] items-center text-sm text-neutral-500">No samples yet.</p>;
  }
  const minT = Math.min(...points.map((p) => p.t));
  const maxT = Math.max(...points.map((p) => p.t));
  const maxV = Math.max(...points.map((p) => p.v), 1);
  const x = (t: number) => pad + ((t - minT) / Math.max(1, maxT - minT)) * (width - pad * 2);
  const y = (v: number) => height - pad - (v / maxV) * (height - pad * 2);

  return (
    <div>
      <svg viewBox={`0 0 ${width} ${height}`} className="h-36 w-full">
        {series.map((item, index) => (
          <polyline
            key={item.label}
            fill="none"
            stroke={COLORS[index % COLORS.length]}
            strokeWidth="2"
            points={item.points.map((p) => `${x(p.t)},${y(p.v)}`).join(" ")}
          />
        ))}
      </svg>
      <ul className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-neutral-400">
        {series.map((item, index) => (
          <li key={item.label} className="inline-flex items-center gap-1">
            <span className="h-2 w-2 rounded-full" style={{ background: COLORS[index % COLORS.length] }} />
            {item.label}
          </li>
        ))}
      </ul>
    </div>
  );
}
