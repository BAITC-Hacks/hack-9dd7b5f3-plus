"use client";

import { X } from "lucide-react";
import { cn } from "@/lib/cn";
import type { CallState } from "@/lib/call-controller";
import { shortId } from "@/lib/format";

export function StatusLine({ st, onDismiss }: { st: CallState; onDismiss: () => void }) {
  const cfg = st.config;
  const ws = st.wsStatus;
  const wsDot = ws === "open" ? "bg-success" : ws === "connecting" ? "bg-warning animate-pulse" : "bg-destructive";
  const sttLabel = cfg
    ? cfg.stt.provider === "mock" || cfg.stt.provider === "browser" || !cfg.stt.provider
      ? `${cfg.stt.provider || "none"} → browser`
      : `${cfg.stt.provider}${cfg.stt.model ? ` (${cfg.stt.model})` : ""}`
    : "…";
  const items: Array<[string, string]> = cfg
    ? [
        ["llm", `${cfg.llm.provider}${cfg.llm.provider !== "mock" && cfg.llm.model ? ` (${cfg.llm.model})` : ""}`],
        ["stt", sttLabel],
        ["tts", `${cfg.tts.provider}${cfg.tts.provider !== "browser" && cfg.tts.model ? ` (${cfg.tts.model})` : ""}`],
        ["fast_path", cfg.fast_path],
        ["router", cfg.router ?? "—"],
      ]
    : [];
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-border px-4 py-2 text-[11px] text-muted-foreground/70">
      <span className="inline-flex items-center gap-1.5" title={`WebSocket ${ws}`}>
        <span className={cn("size-1.5 rounded-full", wsDot)} />
        <span className="font-mono tracking-normal">{st.sessionId ? shortId(st.sessionId, 12) : "no session"}</span>
        <span>· ws {ws}</span>
        {st.restFallback && ws !== "open" && <span className="text-warning">· REST fallback</span>}
      </span>
      {items.map(([k, v]) => (
        <span key={k} className="font-mono text-[10.5px] tracking-normal">
          <span className="text-muted-foreground/45">{k}:</span> {v}
        </span>
      ))}
      {st.status && (
        <span
          className={cn(
            "ml-auto inline-flex max-w-full items-center gap-1.5 rounded-full border px-2 py-0.5",
            st.status.kind === "error" && "border-destructive/30 bg-destructive/10 text-destructive",
            st.status.kind === "warn" && "border-warning/30 bg-warning/10 text-warning",
            st.status.kind === "info" && "border-border bg-foreground/[0.04] text-foreground/80",
          )}
          role={st.status.kind === "error" ? "alert" : "status"}
        >
          <span className="truncate">{st.status.text}</span>
          <button type="button" onClick={onDismiss} aria-label="Dismiss" className="opacity-60 hover:opacity-100">
            <X className="size-3" />
          </button>
        </span>
      )}
    </div>
  );
}
