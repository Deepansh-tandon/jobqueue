import Link from "next/link";
import { cn } from "@/lib/utils";

export function SiteNav({ active }: { active?: "home" | "console" }) {
  const item = (id: "home" | "console", href: string, label: string) => (
    <Link
      href={href}
      className={cn(
        "rounded-full px-3 py-1.5 text-sm",
        active === id
          ? "bg-white/10 font-medium text-white"
          : "text-neutral-400 hover:text-white",
      )}
    >
      {label}
    </Link>
  );

  return (
    <header className="relative z-30 border-b border-white/10 bg-[#07080a]/70 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-5">
        <Link href="/" className="text-sm font-semibold tracking-tight text-white">
          jobqueue
        </Link>
        <nav className="flex items-center gap-1">
          {item("home", "/", "Home")}
          {item("console", "/console", "Console")}
        </nav>
      </div>
    </header>
  );
}
