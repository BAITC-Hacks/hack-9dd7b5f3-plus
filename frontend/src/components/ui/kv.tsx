import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

/** Compact key/value grid for slots, entities, state. */
export function KV({ items, className, mono = true }: { items: Array<[string, ReactNode]>; className?: string; mono?: boolean }) {
  if (items.length === 0) return <p className="text-[12px] text-muted-foreground/40">—</p>;
  return (
    <dl className={cn("grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-[12px]", className)}>
      {items.map(([k, v]) => (
        <div key={k} className="contents">
          <dt className="truncate text-muted-foreground/70">{k}</dt>
          <dd className={cn("min-w-0 break-words text-foreground/90", mono && "font-mono text-[11.5px] tracking-normal")}>{v}</dd>
        </div>
      ))}
    </dl>
  );
}

export function valueText(v: unknown): string {
  if (v === null || v === undefined) return "—";
  if (typeof v === "string") return v;
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  try {
    return JSON.stringify(v);
  } catch {
    return String(v);
  }
}
