import * as React from "react";
import { cn } from "@/lib/utils";

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "h-9 w-full min-w-0 rounded-lg border border-neutral-300 bg-white px-3 py-1 text-sm text-neutral-900 outline-none transition-colors placeholder:text-neutral-500 focus-visible:border-neutral-500 focus-visible:ring-2 focus-visible:ring-neutral-300 disabled:cursor-not-allowed disabled:opacity-50 dark:border-neutral-600 dark:bg-neutral-900 dark:text-neutral-100",
        className,
      )}
      {...props}
    />
  );
}

export { Input };
