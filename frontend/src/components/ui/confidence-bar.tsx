import { cn } from "@/lib/cn";
import { fmtPct } from "@/lib/format";

export interface Thresholds {
  proceed: number;
  clarify: number;
}

export const DEFAULT_THRESHOLDS: Thresholds = { proceed: 0.55, clarify: 0.3 };

export function confidenceTone(v: number | undefined, th: Thresholds = DEFAULT_THRESHOLDS): "success" | "warning" | "destructive" {
  if (v === undefined) return "destructive";
  if (v >= th.proceed) return "success";
  if (v >= th.clarify) return "warning";
  return "destructive";
}

export function ConfidenceBar({
  value,
  thresholds = DEFAULT_THRESHOLDS,
  className,
  showValue = true,
  width = "w-16",
}: {
  value: number | undefined | null;
  thresholds?: Thresholds;
  className?: string;
  showValue?: boolean;
  width?: string;
}) {
  const v = typeof value === "number" && !Number.isNaN(value) ? Math.max(0, Math.min(1, value)) : undefined;
  const tone = confidenceTone(v, thresholds);
  const fill = tone === "success" ? "bg-success" : tone === "warning" ? "bg-warning" : "bg-destructive";
  return (
    <span className={cn("inline-flex items-center gap-2", className)} title={v === undefined ? "no confidence" : `confidence ${fmtPct(v)}`}>
      <span className={cn("relative h-1.5 overflow-hidden rounded-full bg-primary/[0.15]", width)}>
        <span className={cn("absolute inset-y-0 left-0 rounded-full transition-[width] duration-500", fill)} style={{ width: `${Math.round((v ?? 0) * 100)}%` }} />
      </span>
      {showValue && <span className="text-[11px] tabular-nums text-muted-foreground">{v === undefined ? "—" : fmtPct(v)}</span>}
    </span>
  );
}
