import { cn } from "@/lib/cn";
import type { Timings } from "@/lib/types";

export const ROUTE_TARGET_MS = 500;
export const FIRST_AUDIO_TARGET_MS = 1500;

interface Row {
  key: string;
  label: string;
  start: number;
  len: number;
  kind: "span" | "llm" | "point" | "total" | "client";
  target?: number;
  value: number | undefined;
}

function num(t: Timings, k: string): number | undefined {
  const v = t[k];
  return typeof v === "number" && !Number.isNaN(v) ? v : undefined;
}

/** Lays the stage durations out as sequential spans; route / first audio / total are measured from t0. */
export function buildRows(t: Timings, clientE2E?: number): Row[] {
  const rows: Row[] = [];
  let cursor = 0;
  const span = (key: string, label: string) => {
    const v = num(t, key);
    if (v === undefined) return;
    rows.push({ key, label, start: cursor, len: v, kind: "span", value: v });
    cursor += v;
  };
  span("stt", "stt");
  span("triage", "triage");
  span("retrieval", "retrieval");
  span("facts", "facts");
  const llmStart = cursor;
  const ttft = num(t, "llm_ttft");
  if (ttft !== undefined) rows.push({ key: "llm_ttft", label: "llm ttft", start: llmStart, len: ttft, kind: "llm", value: ttft });
  const llmTotal = num(t, "llm_total");
  if (llmTotal !== undefined) {
    rows.push({ key: "llm_total", label: "llm total", start: llmStart, len: llmTotal, kind: "llm", value: llmTotal });
    cursor = llmStart + llmTotal;
  }
  const route = num(t, "route");
  rows.push({ key: "route", label: "route", start: 0, len: route ?? 0, kind: "point", target: ROUTE_TARGET_MS, value: route });
  const fu = num(t, "followup_llm");
  if (fu !== undefined) rows.push({ key: "followup_llm", label: "follow-up llm", start: cursor, len: fu, kind: "llm", value: fu });
  const fa = num(t, "first_audio");
  const ttsFB = num(t, "tts_first_byte");
  if (ttsFB !== undefined) {
    rows.push({ key: "tts_first_byte", label: "tts first byte", start: Math.max(0, (fa ?? route ?? 0) - ttsFB), len: ttsFB, kind: "span", value: ttsFB });
  }
  rows.push({ key: "first_audio", label: "first audio", start: 0, len: fa ?? 0, kind: "point", target: FIRST_AUDIO_TARGET_MS, value: fa });
  if (clientE2E !== undefined) rows.push({ key: "client", label: "client e2e", start: 0, len: clientE2E, kind: "client", target: FIRST_AUDIO_TARGET_MS, value: clientE2E });
  const total = num(t, "total");
  rows.push({ key: "total", label: "total", start: 0, len: total ?? 0, kind: "total", value: total });
  return rows;
}

function fillClass(r: Row): string {
  if (r.kind === "span") return "bg-foreground/30";
  if (r.kind === "llm") return "bg-primary/55";
  if (r.kind === "total") return "bg-foreground/55";
  if (r.kind === "client") return "bg-info";
  if (r.value === undefined) return "bg-foreground/20";
  return r.target !== undefined && r.value > r.target ? "bg-warning" : "bg-success";
}

function valueClass(r: Row): string {
  if (r.value === undefined) return "text-muted-foreground/40";
  if ((r.kind === "point" || r.kind === "client") && r.target !== undefined) return r.value > r.target ? "text-warning" : "text-success";
  return "text-muted-foreground";
}

export function TimingsWaterfall({ timings, clientE2E, compact, className }: { timings: Timings; clientE2E?: number; compact?: boolean; className?: string }) {
  const rows = buildRows(timings, clientE2E);
  const max = Math.max(1700, ...rows.map((r) => r.start + r.len)) * 1.04;
  const pct = (v: number) => `${Math.min(100, (v / max) * 100)}%`;
  const ticks = [0, 500, 1000, 1500, 2000, 3000, 4000, 6000, 8000].filter((t) => t <= max);
  const cols = compact ? "grid-cols-[76px_1fr_46px]" : "grid-cols-[92px_1fr_52px]";
  return (
    <div className={cn("space-y-[5px]", className)}>
      <div className={cn("grid items-end gap-2", cols)}>
        <span className="text-[10.5px] text-muted-foreground/55">stage</span>
        <div className="relative h-3.5">
          {ticks.map((t) => (
            <span key={t} className="absolute -translate-x-1/2 text-[9.5px] tabular-nums text-muted-foreground/45" style={{ left: pct(t) }}>
              {t >= 1000 ? `${t / 1000}s` : t}
            </span>
          ))}
        </div>
        <span className="text-right text-[10.5px] text-muted-foreground/55">ms</span>
      </div>
      {rows.map((r) => (
        <div key={r.key} className={cn("grid items-center gap-2", cols)}>
          <span
            className={cn(
              "truncate text-[11.5px]",
              r.kind === "point" || r.kind === "total" || r.kind === "client" ? "text-foreground/85" : "text-muted-foreground",
            )}
            title={r.target !== undefined ? `target ≤ ${r.target} ms` : undefined}
          >
            {r.label}
            {r.target !== undefined && <span className="ml-1 text-[9.5px] text-muted-foreground/45">≤{r.target}</span>}
          </span>
          <div className="relative h-3 overflow-hidden rounded-[3px] bg-foreground/[0.04]">
            <span className="absolute inset-y-0 border-l border-dashed border-success/45" style={{ left: pct(ROUTE_TARGET_MS) }} />
            <span className="absolute inset-y-0 border-l border-dashed border-info/45" style={{ left: pct(FIRST_AUDIO_TARGET_MS) }} />
            <div
              className={cn("absolute inset-y-[3px] rounded-[2px] transition-[width,left] duration-300", fillClass(r))}
              style={{ left: pct(r.start), width: `max(2px, ${pct(r.len)})` }}
            />
          </div>
          <span className={cn("text-right text-[11.5px] tabular-nums", valueClass(r))}>{r.value === undefined ? "—" : Math.round(r.value)}</span>
        </div>
      ))}
      <p className="pt-1 text-[10px] text-muted-foreground/45">
        dashed lines: <span className="text-success/80">route ≤ 500 ms</span> · <span className="text-info/80">first audio ≤ 1500 ms</span>
        {clientE2E !== undefined && <> · client e2e = end of speech → first sample played (browser clock)</>}
      </p>
    </div>
  );
}
