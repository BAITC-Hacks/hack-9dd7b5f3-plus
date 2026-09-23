"use client";

import { api } from "@/lib/api";
import { cn } from "@/lib/cn";
import { useApi } from "@/lib/use-api";

export function BackendStatus({ compact }: { compact?: boolean }) {
  const { data, error, loading } = useApi(api.health, { interval: 15_000 });
  const ok = !!data?.ok && !error;
  const dot = loading ? "bg-muted-foreground/40" : ok ? "bg-success" : "bg-destructive";
  if (compact) {
    return (
      <span className="inline-flex items-center gap-1.5 text-[11px] text-muted-foreground" title={error ?? (ok ? "backend online" : "backend offline")}>
        <span className={cn("size-1.5 rounded-full", dot)} />
        {loading ? "…" : ok ? "online" : "offline"}
      </span>
    );
  }
  return (
    <div className="space-y-1.5 text-[11px] text-muted-foreground">
      <div className="flex items-center gap-2">
        <span className={cn("size-1.5 rounded-full", dot, ok && "shadow-[0_0_0_3px] shadow-success/20")} />
        <span className="font-medium text-foreground/80">{loading ? "Connecting…" : ok ? "Backend online" : "Backend offline"}</span>
      </div>
      {ok && data && (
        <dl className="grid grid-cols-[auto_1fr] gap-x-2 gap-y-0.5 font-mono text-[10.5px] tracking-normal text-muted-foreground/70">
          <dt>router</dt>
          <dd className="truncate text-foreground/70">{data.router}</dd>
          <dt>stt</dt>
          <dd className="truncate text-foreground/70">{data.stt}</dd>
          <dt>tts</dt>
          <dd className="truncate text-foreground/70">{data.tts}</dd>
        </dl>
      )}
      {error && <p className="truncate text-destructive/80" title={error}>{error}</p>}
      {ok && data?.mock_mode && <p className="text-warning/80">keyless mode (mock LLM)</p>}
    </div>
  );
}
