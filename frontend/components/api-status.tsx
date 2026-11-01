"use client";

import { useEffect, useState } from "react";

type Status = "checking" | "online" | "offline";

export function ApiStatus() {
  const [status, setStatus] = useState<Status>("checking");

  useEffect(() => {
    let cancelled = false;

    async function ping() {
      try {
        const res = await fetch("/api/v1/projects", { method: "GET" });
        if (cancelled) return;
        // 401 = API is up but needs auth
        setStatus(res.status === 401 || res.ok ? "online" : "offline");
      } catch {
        if (!cancelled) setStatus("offline");
      }
    }

    ping();
    const id = setInterval(ping, 8000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  const label =
    status === "checking"
      ? "Checking API…"
      : status === "online"
        ? "API online (127.0.0.1:8080)"
        : "API offline — run go run ./cmd/server in jobqueue/";

  const dot =
    status === "checking"
      ? "bg-amber-400"
      : status === "online"
        ? "bg-emerald-500"
        : "bg-red-500";

  return (
    <div
      className="flex items-center gap-2 rounded-md border border-neutral-200 bg-white px-3 py-2 text-sm dark:border-neutral-700 dark:bg-neutral-900"
      role="status"
    >
      <span className={`h-2.5 w-2.5 shrink-0 rounded-full ${dot}`} />
      <span className="text-neutral-700 dark:text-neutral-200">{label}</span>
    </div>
  );
}
