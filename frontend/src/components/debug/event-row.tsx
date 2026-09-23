"use client";

import { ChevronRight } from "lucide-react";
import { cn } from "@/lib/cn";
import { fmtMs, fmtPct, fmtTime, shortId, truncate } from "@/lib/format";
import type { ActionRecord, FastPathCheck, RetrievalData, RouteData, ShadowData, Signals, Trace, VREvent } from "@/lib/types";
import type { DebugRow } from "@/lib/use-sse";
import { JsonView } from "@/components/ui/json-view";
import { type Tone, toneClass } from "@/components/ui/pill";

function typeTone(type: string): Tone {
  if (type === "route" || type.startsWith("llm")) return "primary";
  if (type === "turn_done" || type.startsWith("reply")) return "success";
  if (type === "action" || type === "handoff" || type === "shadow" || type === "fast_path") return "warning";
  if (type.endsWith("error") || type === "error") return "destructive";
  if (type.startsWith("stt") || type.startsWith("tts") || type === "speak" || type === "audio_end") return "info";
  return "neutral";
}

function str(v: unknown): string {
  return typeof v === "string" ? v : "";
}

export function summarize(ev: VREvent): string {
  const d = ev.data as Record<string, unknown> | undefined;
  switch (ev.type) {
    case "session":
      return `session ${str(d?.session_id)} · audio out ${(d?.audio_out as { sample_rate?: number } | undefined)?.sample_rate ?? "?"} Hz`;
    case "turn_start":
      return `“${str(d?.text)}” · ${str(d?.source)}`;
    case "triage": {
      const s = ev.data as Signals;
      const parts = [`lang ${s.language}`];
      const ents = Object.keys(s.entities ?? {});
      if (ents.length) parts.push(`entities ${ents.join(",")}`);
      if (s.urgent?.length) parts.push(`urgent ${s.urgent.join(",")}`);
      if (s.multi_intent_markers?.length) parts.push(`multi-intent ${s.multi_intent_markers.join(",")}`);
      if (s.confirmation) parts.push(`confirmation ${s.confirmation}`);
      if (s.operator_request) parts.push("operator request");
      if (s.goodbye) parts.push("goodbye");
      return parts.join(" · ");
    }
    case "retrieval": {
      const r = ev.data as RetrievalData;
      return (r.candidates ?? []).map((c) => `${c.id} ${c.score.toFixed(2)}`).join(", ") || "no candidates";
    }
    case "facts":
      return Array.isArray(ev.data) ? (ev.data as ActionRecord[]).map((f) => `${f.name}${f.error ? " ✗" : ""}`).join(", ") : "";
    case "fast_path": {
      const f = ev.data as FastPathCheck;
      return `${f.eligible ? "eligible" : "not taken"} — ${f.reason}`;
    }
    case "llm_start":
      return `${str(d?.model)}${d?.prompt_chars ? ` · ${d.prompt_chars} prompt chars` : ""}${d?.follow_up ? " · follow-up" : ""}`;
    case "llm_delta":
    case "reply_delta":
    case "reply_done":
    case "speak":
    case "stt_partial":
    case "stt_final":
      return str(d?.text);
    case "route": {
      const r = ev.data as RouteData;
      const p = r.decision?.scenarios?.[0];
      return `${p ? `${p.id} ${fmtPct(p.confidence)}` : "no scenario"} · ${r.verdict?.action ?? "?"} · route ${fmtMs(r.route_ms)} (${fmtMs(r.since_t0_ms)} since t0) · reply ${r.language}`;
    }
    case "action": {
      const a = ev.data as ActionRecord;
      return `${a.name} · ${a.mode}${a.error ? ` · ${a.error}` : ""}${a.note ? ` · ${a.note}` : ""}`;
    }
    case "handoff":
      return `queue ${str(d?.queue)}${d?.summary ? ` · ${str(d.summary)}` : ""}`;
    case "tts_first_byte":
      return `tts ${fmtMs(d?.tts_ms as number)} · first audio ${fmtMs(d?.first_audio_ms as number)}`;
    case "tts_sentence":
      return `${d?.bytes ?? 0} bytes · ${fmtMs(d?.ms as number)} · ${truncate(str(d?.text), 60)}`;
    case "stt_start":
      return `${fmtMs(d?.audio_ms as number)} of audio (${d?.bytes ?? 0} bytes)`;
    case "stt_error":
      return `${str(d?.stage)}: ${str(d?.error)}${d?.fallback ? ` → ${str(d.fallback)}` : ""}`;
    case "stt_realtime":
      return str(d?.status);
    case "turn_done": {
      const t = ev.data as Trace;
      const p = t.decision?.scenarios?.[0];
      return `${t.path} · ${p ? `${p.id} ${fmtPct(p.confidence)}` : "no scenario"} · ${t.policy?.action ?? ""} · total ${fmtMs(t.timings?.total)}${t.timings?.first_audio ? ` · first audio ${fmtMs(t.timings.first_audio)}` : ""}`;
    }
    case "shadow": {
      const s = ev.data as ShadowData;
      return `fast ${s.fast_primary} vs llm ${s.shadow?.llm_primary || "?"} — ${s.shadow?.error ? s.shadow.error : s.shadow?.agree ? "agree" : "DISAGREE"} · ${fmtMs(s.shadow?.ms)}`;
    }
    case "turn_error":
    case "error":
    case "llm_error":
    case "tts_error":
      return str(d?.error);
    case "audio_end":
      return "";
    default:
      return d ? truncate(JSON.stringify(d), 120) : "";
  }
}

export function EventRow({ row, expanded, onToggle }: { row: DebugRow; expanded: boolean; onToggle: () => void }) {
  const { ev } = row;
  const tone = typeTone(ev.type);
  const standout = ev.type === "route" || ev.type === "turn_done";
  const isDelta = ev.type === "llm_delta";
  const summary = summarize(ev);
  return (
    <li
      className={cn(
        "border-b border-border/70 text-[12px]",
        standout && "border-l-2 border-l-primary bg-primary/[0.05]",
        (ev.type.endsWith("error") || ev.type === "error") && "bg-destructive/[0.04]",
      )}
    >
      <button type="button" onClick={onToggle} className="grid w-full grid-cols-[14px_84px_112px_minmax(0,1fr)] items-start gap-2 px-3 py-1.5 text-left hover:bg-foreground/[0.03] sm:grid-cols-[14px_84px_112px_90px_minmax(0,1fr)]" aria-expanded={expanded}>
        <ChevronRight className={cn("mt-0.5 size-3.5 text-muted-foreground/50 transition-transform", expanded && "rotate-90")} />
        <span className="font-mono text-[11px] tracking-normal tabular-nums text-muted-foreground/70">{fmtTime(ev.t, true)}</span>
        <span className={cn("inline-flex h-5 w-fit items-center rounded-full border px-2 font-mono text-[10.5px] tracking-normal", toneClass[tone], standout && "font-medium")}>
          {ev.type}
          {row.merged && row.merged > 1 && <span className="ml-1 opacity-70">×{row.merged}</span>}
        </span>
        <span className="hidden font-mono text-[10.5px] tracking-normal text-muted-foreground/60 sm:block">
          {ev.session_id ? shortId(ev.session_id, 8) : ""}
          {ev.turn ? <span className="text-muted-foreground/40"> #{ev.turn}</span> : null}
        </span>
        <span className={cn("min-w-0 leading-snug", isDelta ? "whitespace-pre-wrap break-words font-mono text-[11.5px] tracking-normal text-foreground/80" : "truncate text-foreground/85")}>
          {isDelta ? (
            <>
              {summary}
              <span className="caret" />
            </>
          ) : (
            summary || <span className="text-muted-foreground/40">—</span>
          )}
        </span>
      </button>
      {expanded && (
        <div className="px-3 pb-3 pl-9">
          <JsonView value={ev} maxHeight="max-h-96" />
        </div>
      )}
    </li>
  );
}
