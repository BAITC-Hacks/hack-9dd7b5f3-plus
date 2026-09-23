"use client";
/**
 * Supervisor console — shared primitives (Speko console surface: dense, mono-labelled, hairline-separated).
 */
import type React from "react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { ActionCall, ActionMode, LatencyMs, PolicyAction, PolicyVerdict, RouterDecision, Stage, Trace } from "@/lib/contract";
import type { LiveTurn } from "@/lib/store";

/** Normalised view of the turn shown "под капотом": live in-progress turn, else the last finished trace. */
export interface TurnView {
  source: "live" | "trace" | "none";
  turn: number;
  stage: Stage | "done" | null;
  transcript: string;
  language?: string;
  parts?: string[];
  urgent?: boolean;
  candidates: { scenario_id: string; confidence: number }[];
  decision?: RouterDecision;
  verdict?: PolicyVerdict;
  actions: ActionCall[];
  latency: LatencyMs;
  error?: string;
}

export function toTurnView(live: LiveTurn | null, lastTrace: Trace | undefined): TurnView {
  if (live) {
    return {
      source: "live",
      turn: live.turn,
      stage: live.stage,
      transcript: live.transcript,
      language: live.language,
      parts: live.parts,
      urgent: live.urgent,
      candidates: live.candidates,
      decision: live.decision,
      verdict: live.verdict,
      actions: live.actions,
      latency: live.latency,
      error: live.error,
    };
  }
  if (lastTrace) {
    const t = lastTrace;
    return {
      source: "trace",
      turn: t.turn,
      stage: "done",
      transcript: t.transcript,
      language: t.language,
      candidates: t.scenarios.map((s) => ({ scenario_id: s.scenario_id, confidence: s.confidence })).concat(t.alternatives),
      decision: {
        scenarios: t.scenarios,
        alternatives: t.alternatives,
        language: t.language,
        slots: t.slots,
        is_continuation: t.policy.action === "continue",
        reason: t.reason,
        model: t.model,
        tier: t.tier,
      },
      verdict: t.policy,
      actions: t.actions.map((a) => {
        const [name, mode] = a.split(":");
        return { name, mode: (mode as ActionMode) || "read" };
      }),
      latency: t.latency_ms,
    };
  }
  return { source: "none", turn: 0, stage: null, transcript: "", candidates: [], actions: [], latency: {} };
}

export const STAGES: Stage[] = ["stt", "triage", "router", "policy", "executor", "response", "tts_first_audio", "total"];
export const STAGE_LABEL: Record<Stage, string> = {
  stt: "stt",
  triage: "triage",
  router: "router",
  policy: "policy",
  executor: "executor",
  response: "response",
  tts_first_audio: "tts · first audio",
  total: "total",
};
export const TARGET_MS = 1500;

export function fmtMs(v: number | undefined | null): string {
  return typeof v === "number" && Number.isFinite(v) ? Math.round(v).toLocaleString("ru-RU") : "—";
}
export function fmtPct(v: number | undefined | null): string {
  return typeof v === "number" && Number.isFinite(v) ? `${Math.round(v * 100)} %` : "—";
}

export const POLICY_LABEL: Record<PolicyAction, string> = {
  run: "запуск",
  continue: "продолжение",
  clarify: "уточнение",
  handoff: "оператор",
  out_of_scope: "вне компетенции",
  goodbye: "завершение",
};
const POLICY_VARIANT: Record<PolicyAction, "success" | "info" | "warning" | "error" | "secondary"> = {
  run: "success",
  continue: "info",
  clarify: "warning",
  handoff: "error",
  out_of_scope: "secondary",
  goodbye: "secondary",
};

export function PolicyBadge({ action, size = "default" }: { action?: PolicyAction | null; size?: "sm" | "default" | "lg" }) {
  if (!action) return <span className="text-muted-foreground">—</span>;
  return (
    <Badge variant={POLICY_VARIANT[action]} size={size} className="font-mono tracking-[.06em]">
      {POLICY_LABEL[action]}
    </Badge>
  );
}

export function ModeBadge({ mode }: { mode: ActionMode }) {
  const v = mode === "execute" ? "success" : mode === "preview" ? "warning" : "secondary";
  return (
    <Badge variant={v} size="sm" className="font-mono tracking-[.06em]">
      {mode}
    </Badge>
  );
}

/** Mono chip (provider:model, tier). */
export function MonoChip({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <Badge variant="outline" size="sm" className={cn("font-mono tracking-[.04em]", className)}>
      {children}
    </Badge>
  );
}

/** Card with a mono header line: "[ 02 / 07 ]  КАНДИДАТЫ" + optional right slot. */
export function Section({
  index,
  title,
  right,
  children,
  className,
  bodyClassName,
}: {
  index?: string;
  title: string;
  right?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  bodyClassName?: string;
}) {
  return (
    <Card className={cn("rounded-xl shadow-none before:hidden", className)}>
      <div className="hairline-b flex items-center justify-between gap-3 px-4 py-2.5">
        <div className="marker marker-dot truncate">
          {index ? `[ ${index} ] ` : ""}
          {title}
        </div>
        {right && <div className="flex shrink-0 items-center gap-2">{right}</div>}
      </div>
      <div className={cn("p-4", bodyClassName)}>{children}</div>
    </Card>
  );
}

/** Horizontal confidence bar (6px). */
export function Bar({ value, fill = "bg-s6", track = "bg-s2", className, animate = true }: { value: number; fill?: string; track?: string; className?: string; animate?: boolean }) {
  const w = Math.max(0, Math.min(100, Math.round(value * 100)));
  return (
    <div className={cn("h-1.5 w-full overflow-hidden rounded-xs", track, className)}>
      <div className={cn("h-full rounded-xs", fill, animate && "transition-[width] duration-200 ease-out")} style={{ width: `${w}%` }} />
    </div>
  );
}

/** Key / value row, hairline-separated. */
export function KV({ k, children, className }: { k: string; children: React.ReactNode; className?: string }) {
  return (
    <div className={cn("hairline-b flex items-baseline justify-between gap-4 py-2 last:border-b-0", className)}>
      <span className="marker shrink-0">{k}</span>
      <span className="min-w-0 truncate text-right text-sm">{children}</span>
    </div>
  );
}

export function Dash() {
  return <span className="text-muted-foreground">—</span>;
}

export function MetricCard({ label, value, unit, description }: { label: string; value: string; unit?: string; description?: string }) {
  return (
    <Card className="rounded-xl shadow-none before:hidden">
      <div className="flex flex-col gap-1.5 p-4">
        <div className="metric-label">{label}</div>
        <div className="metric-value">
          {value}
          {unit && <span className="ml-1 text-base font-normal text-muted-foreground">{unit}</span>}
        </div>
        {description && <div className="text-xs leading-snug text-muted-foreground">{description}</div>}
      </div>
    </Card>
  );
}

/** Trace in README format (turn, transcript, language, scenarios, alternatives, reason, slots, actions, latency_ms). */
export function readmeTrace(t: Trace) {
  return {
    turn: t.turn,
    transcript: t.transcript,
    language: t.language,
    scenarios: t.scenarios,
    alternatives: t.alternatives,
    reason: t.reason,
    slots: t.slots,
    actions: t.actions,
    latency_ms: t.latency_ms,
  };
}
