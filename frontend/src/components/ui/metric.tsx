import type { ReactNode } from "react";
import { cn } from "@/lib/cn";
import { Label } from "./panel";

export function Metric({
  label,
  value,
  unit,
  foot,
  tone,
  size = "md",
}: {
  label: ReactNode;
  value: ReactNode;
  unit?: ReactNode;
  foot?: ReactNode;
  tone?: "success" | "warning" | "destructive" | "primary";
  size?: "md" | "sm";
}) {
  const toneClass =
    tone === "success"
      ? "text-success"
      : tone === "warning"
        ? "text-warning"
        : tone === "destructive"
          ? "text-destructive"
          : tone === "primary"
            ? "text-primary"
            : "text-foreground";
  return (
    <div className="min-w-0 px-4 py-3.5 sm:px-5">
      <Label className="truncate">{label}</Label>
      <p className="mt-1.5 flex items-baseline gap-1.5">
        <span
          className={cn(
            "font-[450] leading-none tracking-[-0.035em] tabular-nums",
            size === "md" ? "text-[28px]" : "text-[22px]",
            toneClass,
          )}
        >
          {value}
        </span>
        {unit && <span className="text-[11px] text-muted-foreground/55">{unit}</span>}
      </p>
      {foot && <p className="mt-1.5 truncate text-[11.5px] tabular-nums text-muted-foreground/65">{foot}</p>}
    </div>
  );
}

/** One PANEL holding metric cells, hairline-divided, 2-up on phones and `cols`-up on desktop. */
export function MetricGrid({ children, className, cols = 4 }: { children: ReactNode; className?: string; cols?: 2 | 3 | 4 }) {
  const layout =
    cols === 2
      ? "grid-cols-2 [&>*:nth-child(even)]:border-l"
      : cols === 3
        ? "grid-cols-2 sm:grid-cols-3 [&>*:nth-child(even)]:border-l [&>*:nth-child(n+3)]:border-t sm:[&>*:nth-child(even)]:border-l-0 sm:[&>*:not(:first-child)]:border-l sm:[&>*:nth-child(n+3)]:border-t-0 sm:[&>*:nth-child(n+4)]:border-t"
        : "grid-cols-2 lg:grid-cols-4 [&>*:nth-child(even)]:border-l [&>*:nth-child(n+3)]:border-t lg:[&>*:nth-child(even)]:border-l-0 lg:[&>*:not(:first-child)]:border-l lg:[&>*:nth-child(n+3)]:border-t-0 lg:[&>*:nth-child(n+5)]:border-t";
  return <div className={cn("grid rounded-[15px] border border-border bg-card [&>*]:border-border", layout, className)}>{children}</div>;
}
