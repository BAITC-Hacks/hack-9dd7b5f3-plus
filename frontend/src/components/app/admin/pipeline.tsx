"use client";
/** Pipeline stepper + per-stage latency waterfall. */
import { cn } from "@/lib/utils";
import type { LatencyMs, Stage } from "@/lib/contract";
import { Dash, fmtMs, Section, STAGE_LABEL, STAGES, TARGET_MS, type TurnView } from "./shared";

type StepState = "done" | "active" | "pending";

function stepState(stage: Stage, current: TurnView["stage"], latency: LatencyMs): StepState {
  if (current === null) return "pending";
  if (current === "done") return "done";
  if (stage === "total") return "pending";
  const idx = STAGES.indexOf(stage);
  const cur = STAGES.indexOf(current);
  if (typeof latency[stage] === "number" || idx < cur) return "done";
  if (idx === cur) return "active";
  return "pending";
}

export function PipelineStepper({ view }: { view: TurnView }) {
  return (
    <Section index="01 / 07" title="Конвейер" right={<span className="marker">{view.source === "live" ? `ход ${view.turn} · live` : view.source === "trace" ? `ход ${view.turn} · trace` : "нет данных"}</span>}>
      <ol className="grid grid-cols-2 gap-x-4 gap-y-2 sm:grid-cols-4">
        {STAGES.map((st) => {
          const state = stepState(st, view.stage, view.latency);
          const ms = view.latency[st];
          return (
            <li key={st} className={cn("hairline-b flex items-center justify-between gap-2 pb-2", st === "total" && "font-medium")}>
              <span className="flex min-w-0 items-center gap-2">
                <span
                  className={cn(
                    "inline-block size-1.5 shrink-0 rounded-full",
                    state === "done" && "bg-success-foreground",
                    state === "active" && "animate-pulse bg-brand",
                    state === "pending" && "bg-muted-foreground/40",
                  )}
                />
                <span className={cn("marker truncate normal-case tracking-[.06em]", state === "pending" && "opacity-60", state !== "pending" && "text-foreground")}>{STAGE_LABEL[st]}</span>
              </span>
              <span className="num text-xs">{typeof ms === "number" ? fmtMs(ms) : state === "active" ? "…" : "—"}</span>
            </li>
          );
        })}
      </ol>
    </Section>
  );
}

const WATERFALL: { stage: Stage; color: string }[] = [
  { stage: "stt", color: "bg-s3" },
  { stage: "triage", color: "bg-s4" },
  { stage: "router", color: "bg-s6" },
  { stage: "policy", color: "bg-s4" },
  { stage: "executor", color: "bg-s5" },
  { stage: "response", color: "bg-s6" },
  { stage: "tts_first_audio", color: "bg-s7" },
];

export function LatencyWaterfall({ latency }: { latency: LatencyMs }) {
  const total = latency.total;
  const sumStages = WATERFALL.reduce((a, w) => a + (latency[w.stage] ?? 0), 0);
  const scale = Math.max(TARGET_MS, total ?? 0, sumStages) * 1.08;
  const markerLeft = (TARGET_MS / scale) * 100;
  let cursor = 0;
  const hasAny = sumStages > 0 || typeof total === "number";

  return (
    <Section
      index="07 / 07"
      title="Задержка по этапам"
      right={
        <span className="marker">
          total <span className={cn("num", typeof total === "number" && total > TARGET_MS ? "text-warning-foreground" : "text-foreground")}>{typeof total === "number" ? `${fmtMs(total)} ms` : "—"}</span>
        </span>
      }
      bodyClassName="p-0"
    >
      {!hasAny ? (
        <div className="p-4 text-sm text-muted-foreground">Нет замеров — <Dash /></div>
      ) : (
        <div className="relative">
          {/* target marker */}
          <div className="pointer-events-none absolute inset-y-0 z-10" style={{ left: `calc(112px + (100% - 112px - 64px) * ${markerLeft / 100})` }}>
            <div className="h-full w-px border-l border-dashed border-warning-foreground/70" />
            <span className="absolute -top-0.5 left-1.5 whitespace-nowrap font-mono text-[10px] tracking-[.08em] text-warning-foreground">1 500 ms · ориентир +2 балла</span>
          </div>
          <div className="divide-y divide-border">
            {WATERFALL.map(({ stage, color }) => {
              const ms = latency[stage];
              const left = (cursor / scale) * 100;
              const width = typeof ms === "number" ? (ms / scale) * 100 : 0;
              if (typeof ms === "number") cursor += ms;
              return (
                <div key={stage} className="flex items-center gap-2 px-4 py-2 first:pt-5">
                  <span className="marker w-[96px] shrink-0 truncate normal-case tracking-[.06em]">{STAGE_LABEL[stage]}</span>
                  <div className="relative h-2.5 min-w-0 flex-1 overflow-hidden rounded-xs bg-s0/10 dark:bg-white/[.04]">
                    <div className={cn("absolute inset-y-0 rounded-xs transition-[width,left] duration-300", color)} style={{ left: `${left}%`, width: `${Math.max(width, typeof ms === "number" && ms > 0 ? 0.6 : 0)}%` }} />
                  </div>
                  <span className="num w-[56px] shrink-0 text-xs">{typeof ms === "number" ? fmtMs(ms) : "—"}</span>
                </div>
              );
            })}
            <div className="flex items-center gap-2 px-4 py-2 font-medium">
              <span className="marker w-[96px] shrink-0 text-foreground">total</span>
              <div className="relative h-2.5 min-w-0 flex-1 overflow-hidden rounded-xs">
                {typeof total === "number" && <div className="absolute inset-y-0 left-0 rounded-xs bg-foreground/25" style={{ width: `${Math.min(100, (total / scale) * 100)}%` }} />}
              </div>
              <span className="num w-[56px] shrink-0 text-xs">{typeof total === "number" ? fmtMs(total) : "—"}</span>
            </div>
          </div>
          <div className="hairline-t px-4 py-2 text-xs text-muted-foreground">от конца реплики клиента до первого звука ответа · мок-замеры, не результат нагрузочного теста</div>
        </div>
      )}
    </Section>
  );
}
