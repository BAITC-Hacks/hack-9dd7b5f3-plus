"use client";
/** Router decision + explainability (description, not_this_if rules). */
import { Badge } from "@/components/ui/badge";
import { isSystemIntent, scenarioById, scenarioLabel, systemIntentById } from "@/lib/catalog";
import { PriorityChip } from "./candidates";
import { Bar, Dash, fmtPct, MonoChip, Section, type TurnView } from "./shared";

export function DecisionCard({ view }: { view: TurnView }) {
  const d = view.decision;
  const top = d?.scenarios[0];
  const sc = top ? scenarioById(top.scenario_id) : undefined;
  const sys = top && isSystemIntent(top.scenario_id) ? systemIntentById(top.scenario_id) : undefined;

  return (
    <Section
      index="03 / 07"
      title="Решение"
      right={
        d ? (
          <>
            {d.model && <MonoChip>{d.model}</MonoChip>}
            {d.tier && <MonoChip>{d.tier}</MonoChip>}
            {d.is_continuation && <Badge variant="info" size="sm" className="font-mono tracking-[.06em]">continuation</Badge>}
          </>
        ) : (
          <span className="marker">{view.stage && view.stage !== "done" ? "ожидание…" : "—"}</span>
        )
      }
      bodyClassName="p-0"
    >
      {!d || !top ? (
        <div className="p-4 text-sm text-muted-foreground">Маршрутизатор ещё не вынес решение — <Dash /></div>
      ) : (
        <>
          <div className="hairline-b flex flex-wrap items-end justify-between gap-4 p-4">
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="font-mono text-xs text-muted-foreground">{top.scenario_id}</span>
                <PriorityChip id={top.scenario_id} />
              </div>
              <div className="h-display mt-1 text-2xl">{scenarioLabel(top.scenario_id, "ru")}</div>
              {sc && <div className="mt-0.5 font-mono text-[11px] text-muted-foreground">{sc.domain} · {sc.category} · {sc.slug}</div>}
            </div>
            <div className="text-right">
              <div className="metric-label">уверенность</div>
              <div className="metric-value">{fmtPct(top.confidence)}</div>
            </div>
          </div>

          <div className="hairline-b p-4">
            <div className="marker mb-1">почему</div>
            <p className="text-sm leading-relaxed">{d.reason || top.reason || <Dash />}</p>
          </div>

          {d.scenarios.length > 1 && (
            <div className="hairline-b p-4">
              <div className="marker mb-2">несколько намерений · порядок исполнения</div>
              <ol className="space-y-1.5">
                {d.scenarios.map((s, i) => (
                  <li key={s.scenario_id} className="flex items-center justify-between gap-3 text-sm">
                    <span className="flex min-w-0 items-center gap-2">
                      <span className="marker">{i + 1}</span>
                      <span className="font-mono text-xs">{s.scenario_id}</span>
                      <span className="truncate">{scenarioLabel(s.scenario_id, "ru")}</span>
                      <PriorityChip id={s.scenario_id} />
                    </span>
                    <span className="num text-xs">{fmtPct(s.confidence)}</span>
                  </li>
                ))}
              </ol>
            </div>
          )}

          {d.alternatives.length > 0 && (
            <div className="hairline-b p-4">
              <div className="marker mb-2">альтернативы · отвергнуты</div>
              <ul className="space-y-2">
                {d.alternatives.slice(0, 4).map((a) => (
                  <li key={a.scenario_id} className="grid grid-cols-[minmax(0,1fr)_56px] items-center gap-x-3 gap-y-1">
                    <span className="flex min-w-0 items-center gap-2 text-sm">
                      <span className="font-mono text-xs text-muted-foreground">{a.scenario_id}</span>
                      <span className="truncate">{scenarioLabel(a.scenario_id, "ru")}</span>
                    </span>
                    <span className="num text-xs text-muted-foreground">{fmtPct(a.confidence)}</span>
                    <Bar value={a.confidence} fill="bg-s4" animate={false} />
                  </li>
                ))}
              </ul>
            </div>
          )}

          {/* Explainability */}
          <div className="p-4">
            <div className="marker mb-2">explainability · правила из каталога</div>
            {sc ? (
              <>
                <p className="text-sm leading-relaxed text-muted-foreground">{sc.description}</p>
                {sc.not_this_if.length > 0 && (
                  <ul className="mt-3 divide-y divide-border">
                    {sc.not_this_if.map((r, i) => (
                      <li key={i} className="flex items-start justify-between gap-3 py-2 text-sm">
                        <span className="min-w-0">
                          <span className="font-mono text-[11px] uppercase tracking-[.1em] text-down">не этот, если</span> {r.condition}
                        </span>
                        <span className="flex shrink-0 items-center gap-1.5 font-mono text-xs">
                          <span className="text-muted-foreground">→</span>
                          <span>{r.use_instead}</span>
                          <span className="hidden text-muted-foreground xl:inline">· {scenarioLabel(r.use_instead, "ru")}</span>
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
                <div className="mt-3 flex flex-wrap gap-1.5">
                  {sc.requires_identification && <MonoChip>identification</MonoChip>}
                  {sc.requires_confirmation && <MonoChip>confirmation</MonoChip>}
                  {sc.fast_path_eligible && <MonoChip>fast-path</MonoChip>}
                  {sc.handoff && <MonoChip>handoff → {sc.handoff.queue}</MonoChip>}
                </div>
              </>
            ) : sys ? (
              <>
                <p className="text-sm leading-relaxed text-muted-foreground">{sys.description}</p>
                <p className="mt-2 text-sm">{sys.behavior}</p>
              </>
            ) : (
              <Dash />
            )}
          </div>
        </>
      )}
    </Section>
  );
}
