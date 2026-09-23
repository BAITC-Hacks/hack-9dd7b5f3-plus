"use client";
/** Live router candidates: confidence bars converging in real time. */
import { Badge } from "@/components/ui/badge";
import { scenarioById, scenarioLabel } from "@/lib/catalog";
import { Bar, Dash, fmtPct, Section, type TurnView } from "./shared";

export function PriorityChip({ id }: { id: string }) {
  const p = scenarioById(id)?.priority;
  if (p === "urgent") return <Badge variant="error" size="sm" className="font-mono tracking-[.06em]">urgent</Badge>;
  if (p === "high") return <Badge variant="warning" size="sm" className="font-mono tracking-[.06em]">high</Badge>;
  return null;
}

export function LiveCandidates({ view }: { view: TurnView }) {
  const partial = view.stage === "router";
  const list = [...view.candidates].sort((a, b) => b.confidence - a.confidence).slice(0, 5);
  return (
    <Section
      index="02 / 07"
      title="Кандидаты в реальном времени"
      right={
        <>
          {view.urgent && <Badge variant="error" size="sm" className="font-mono tracking-[.06em]">urgent</Badge>}
          {partial ? (
            <span className="marker flex items-center gap-1.5 text-brand"><span className="inline-block size-1.5 animate-pulse bg-brand" />partial</span>
          ) : (
            <span className="marker">{list.length ? "final" : "—"}</span>
          )}
        </>
      }
    >
      {list.length === 0 ? (
        <div className="text-sm text-muted-foreground">Ждём реплику — кандидаты появятся во время маршрутизации.</div>
      ) : (
        <ul className="space-y-2.5">
          {list.map((c, i) => (
            <li key={c.scenario_id} className="grid grid-cols-[20px_minmax(0,1fr)_56px] items-center gap-x-3 gap-y-1">
              <span className="marker">{String(i + 1).padStart(2, "0")}</span>
              <div className="flex min-w-0 items-center gap-2">
                <span className="font-mono text-xs">{c.scenario_id}</span>
                <span className="truncate text-sm">{scenarioLabel(c.scenario_id, "ru")}</span>
                <PriorityChip id={c.scenario_id} />
              </div>
              <span className="num text-sm">{fmtPct(c.confidence)}</span>
              <span />
              <Bar value={c.confidence} fill={i === 0 ? "bg-s6" : "bg-s4"} className="col-span-2" />
            </li>
          ))}
        </ul>
      )}
      {view.parts && view.parts.length > 1 && (
        <div className="hairline-t mt-3 pt-2 text-xs text-muted-foreground">
          triage: {view.parts.length} части — {view.parts.map((p) => `«${p}»`).join(" · ")}
        </div>
      )}
      {view.transcript ? <div className="hairline-t mt-3 pt-2 text-xs text-muted-foreground">транскрипт: <span className="text-foreground">{view.transcript}</span></div> : view.source === "none" ? <div className="mt-3 text-xs"><Dash /></div> : null}
    </Section>
  );
}
