"use client";

import { useState, type ReactNode } from "react";
import { ChevronRight } from "lucide-react";
import { cn } from "@/lib/cn";

export function Collapsible({
  title,
  count,
  defaultOpen = false,
  children,
  right,
  className,
  size = "md",
}: {
  title: ReactNode;
  count?: ReactNode;
  defaultOpen?: boolean;
  children: ReactNode;
  right?: ReactNode;
  className?: string;
  size?: "sm" | "md";
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div className={cn("rounded-[10px] border border-border", className)}>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        className={cn(
          "flex w-full items-center gap-2 rounded-[10px] text-left text-foreground/80 hover:bg-foreground/[0.03]",
          size === "md" ? "px-3 py-2 text-[12px]" : "px-2.5 py-1.5 text-[11.5px]",
        )}
      >
        <ChevronRight className={cn("size-3.5 shrink-0 text-muted-foreground/60 transition-transform", open && "rotate-90")} />
        <span className="min-w-0 flex-1 truncate">{title}</span>
        {count !== undefined && <span className="tabular-nums text-muted-foreground/60">{count}</span>}
        {right}
      </button>
      {open && <div className={cn("border-t border-border", size === "md" ? "px-3 py-2.5" : "px-2.5 py-2")}>{children}</div>}
    </div>
  );
}
