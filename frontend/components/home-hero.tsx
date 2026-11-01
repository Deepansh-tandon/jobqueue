"use client";

import Link from "next/link";
import { BackgroundBeams } from "@/components/ui/background-beams";
import { Spotlight } from "@/components/ui/spotlight";
import { SiteNav } from "@/components/site-nav";

const features = [
  ["Register", "Email and password. The API returns a key used as Bearer auth."],
  ["Projects", "Every job and failure belongs to a project you own."],
  ["Enqueue", "echo, sleep, fail_until_n, always_fail, email, receipt, summarize."],
  ["Idempotency", "The same Idempotency-Key replays the same job id."],
  ["Timeline", "queued, claimed, running, completed, failed, requeued."],
  ["Dead letter", "Incidents grouped by fingerprint. Requeue or discard."],
];

export function HomeHero() {
  return (
    <div className="relative min-h-screen overflow-hidden bg-[#07080a] text-white">
      <Spotlight className="-top-40 left-0 md:-top-20 md:left-60" fill="white" />
      <BackgroundBeams className="opacity-50" />
      <div className="relative z-10 flex min-h-screen flex-col">
        <SiteNav active="home" />
        <main className="mx-auto flex w-full max-w-6xl flex-1 flex-col gap-16 px-5 py-16">
          <section className="max-w-3xl">
            <p className="text-xs tracking-[0.28em] text-neutral-500 uppercase">
              jobqueue console
            </p>
            <h1 className="mt-5 text-5xl leading-[1.05] font-semibold tracking-tight md:text-7xl">
              Lease workers.
              <br />
              Idempotent submit.
              <br />
              Failure incidents.
            </h1>
            <p className="mt-6 max-w-xl text-base leading-7 text-neutral-400">
              Operator UI for the Go queue. Workers hold Redis stream leases,
              submits are idempotent, and permanent failures land as incidents
              you can inspect, requeue, or discard.
            </p>
            <div className="mt-8 flex flex-wrap gap-3">
              <Link
                href="/console"
                className="inline-flex h-11 items-center rounded-full bg-white px-5 text-sm font-medium text-black hover:bg-neutral-200"
              >
                Open console
              </Link>
              <Link
                href="/console#account"
                className="inline-flex h-11 items-center rounded-full border border-white/20 bg-black/30 px-5 text-sm font-medium text-white hover:bg-white/10"
              >
                Connect API key
              </Link>
            </div>
          </section>

          <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {features.map(([title, description]) => (
              <li
                key={title}
                className="rounded-2xl border border-white/10 bg-black/40 p-5 backdrop-blur-sm"
              >
                <h2 className="font-medium text-white">{title}</h2>
                <p className="mt-2 text-sm leading-6 text-neutral-400">{description}</p>
              </li>
            ))}
          </ul>
        </main>
      </div>
    </div>
  );
}
