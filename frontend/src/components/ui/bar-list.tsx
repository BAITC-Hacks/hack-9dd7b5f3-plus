import type { ReactNode } from "react";
import { cn } from "@/lib/cn";
import { fmtNum } from "@/lib/format";

export interface BarItem {
  key: string;
  label: ReactNode;
  value: number;
  hint?: ReactNode;
  tone?: "primary" | "success" | "warning" | "destructive" | "info" | "neutral";
}

const fills: Record<NonNullable<BarItem["tone"]>, string> = {
  primary: "bg-primary",
  success: "bg-success",
  warning: "bg-warning",
  destructive: "bg-destructive",
  info: "bg-info",
  neutral: "bg-foreground/35",
};

export function BarList({ items, max, className, format = fmtNum }: { items: BarItem[]; max?: number; className?: string; format?: (v: number) => string }) {
  const top = max ?? Math.max(1, ...items.map((i) => i.value));
  return (
    <ul className={cn("space-y-2", className)}>
      {items.map((it) => (
        <li key={it.key} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1">
          <div className="flex min-w-0 items-center justify-between gap-2">
            <span className="min-w-0 truncate text-[12px] text-foreground/85">{it.label}</span>
            {it.hint && <span className="shrink-0 text-[10.5px] text-muted-foreground/60">{it.hint}</span>}
          </div>
          <span className="w-12 text-right text-[12px] tabular-nums text-muted-foreground">{format(it.value)}</span>
          <div className="col-span-2 h-1.5 overflow-hidden rounded-full bg-foreground/[0.07]">
            <div className={cn("h-full rounded-full transition-[width] duration-700", fills[it.tone ?? "primary"])} style={{ width: `${Math.max(1, Math.round((it.value / top) * 100))}%` }} />
          </div>
        </li>
      ))}
    </ul>
  );
}
