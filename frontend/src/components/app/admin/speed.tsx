"use client";
/** «Скорость»: per-stage latency waterfall with the 1.5 s target. */
import type { LatencyMs, Stage } from "@/lib/contract";
import { cn } from "@/lib/utils";
import { fmtMs, Section, TARGET_MS } from "./shared";

const ROWS: { stage: Stage; label: string; color: string }[] = [
  { stage: "stt", label: "Распознавание", color: "bg-s3" },
  { stage: "triage", label: "Триаж", color: "bg-s4" },
  { stage: "router", label: "Выбор сценария", color: "bg-s6" },
  { stage: "policy", label: "Политика", color: "bg-s4" },
  { stage: "executor", label: "Действия", color: "bg-s5" },
  { stage: "response", label: "Ответ", color: "bg-s6" },
  { stage: "tts_first_audio", label: "Озвучка", color: "bg-s7" },
];

export function SpeedCard({ latency, live }: { latency: LatencyMs; live: boolean }) {
  const total = latency.total;
  const sum = ROWS.reduce((a, r) => a + (latency[r.stage] ?? 0), 0);
  const scale = Math.max(TARGET_MS, total ?? 0, sum) * 1.1;
  const hasAny = sum > 0 || typeof total === "number";
  const over = typeof total === "number" && total > TARGET_MS;

  return (
    <Section
      title="Скорость"
      hint="от конца реплики до первого звука ответа"
      right={
        hasAny ? (
          <span className={cn("text-lg font-medium tabular-nums", over ? "text-warning-foreground" : "text-success-foreground")}>
            {typeof total === "number" ? `${fmtMs(total)} мс` : live ? "…" : "—"}
          </span>
        ) : undefined
      }
    >
      {!hasAny ? (
        <div className="py-2 text-sm text-muted-foreground">Появится после первой реплики.</div>
      ) : (
        <div className="relative">
          <div className="pointer-events-none absolute inset-y-0 z-10" style={{ left: `calc(112px + (100% - 112px - 56px) * ${TARGET_MS / scale})` }}>
            <div className="h-full w-px border-l border-dashed border-muted-foreground/50" />
          </div>
          <div className="space-y-1.5">
            {ROWS.map(({ stage, label, color }, index) => {
              const ms = latency[stage];
              const left = (ROWS.slice(0, index).reduce((sum, row) => sum + (latency[row.stage] ?? 0), 0) / scale) * 100;
              const width = typeof ms === "number" ? (ms / scale) * 100 : 0;
              return (
                <div key={stage} className="flex items-center gap-2 text-sm">
                  <span className="w-[104px] shrink-0 text-muted-foreground">{label}</span>
                  <div className="relative h-2 min-w-0 flex-1 overflow-hidden rounded-full bg-white/[.05]">
                    <div className={cn("absolute inset-y-0 rounded-full transition-[width,left] duration-300", color)} style={{ left: `${left}%`, width: `${Math.max(width, typeof ms === "number" && ms > 0 ? 0.8 : 0)}%` }} />
                  </div>
                  <span className="w-[48px] shrink-0 text-right font-mono text-xs tabular-nums text-muted-foreground">{typeof ms === "number" ? fmtMs(ms) : "—"}</span>
                </div>
              );
            })}
          </div>
          <div className="mt-2 text-xs text-muted-foreground">Пунктир — ориентир {fmtMs(TARGET_MS)} мс (+2 балла жюри). В режиме LLM учитывается запрос к ядру и сети.</div>
        </div>
      )}
    </Section>
  );
}
