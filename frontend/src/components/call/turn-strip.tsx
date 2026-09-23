"use client";

import { cn } from "@/lib/cn";
import { fmtPct } from "@/lib/format";
import type { TurnEntry } from "@/lib/call-controller";
import { Button } from "@/components/ui/button";

export function TurnStrip({ turns, selectedTurn, onSelect }: { turns: TurnEntry[]; selectedTurn: number | null; onSelect: (t: number | null) => void }) {
  if (turns.length === 0) return null;
  const last = turns[turns.length - 1]?.turn ?? null;
  const active = selectedTurn ?? last;
  return (
    <div className="flex items-center gap-2">
      <div className="flex min-w-0 flex-1 gap-1.5 overflow-x-auto pb-1">
        {turns.map((t) => {
          const p = t.trace.decision?.scenarios?.[0];
          const isActive = active === t.turn;
          return (
            <button
              key={t.turn}
              type="button"
              onClick={() => onSelect(t.turn)}
              aria-pressed={isActive}
              className={cn(
                "flex shrink-0 items-center gap-1.5 rounded-full border px-2.5 py-1 text-[11px] transition-colors active:scale-[0.97]",
                isActive ? "border-primary/40 bg-primary/[0.12] text-primary" : "border-border text-muted-foreground hover:bg-foreground/[0.04]",
                t.error && "border-destructive/40",
              )}
              title={t.transcript}
            >
              <span className="tabular-nums">#{t.turn}</span>
              {p ? (
                <>
                  <span className="font-mono tracking-normal">{p.id}</span>
                  <span className="tabular-nums opacity-70">{fmtPct(p.confidence)}</span>
                </>
              ) : (
                <span className="opacity-70">{t.done ? "—" : "…"}</span>
              )}
              {!t.done && !t.error && <span className="size-1.5 animate-pulse rounded-full bg-primary" />}
            </button>
          );
        })}
      </div>
      {selectedTurn !== null && selectedTurn !== last && (
        <Button size="sm" variant="ghost" onClick={() => onSelect(null)} className="shrink-0">
          Follow latest
        </Button>
      )}
    </div>
  );
}
