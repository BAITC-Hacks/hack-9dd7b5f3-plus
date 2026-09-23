"use client";

import Link from "next/link";
import { useMemo } from "react";
import { RefreshCw } from "lucide-react";
import { api } from "@/lib/api";
import { useScenarioNames } from "@/lib/catalog-cache";
import { fmtDateTime, fmtNum, fmtPct, ratio, shortId, truncate } from "@/lib/format";
import type { Percentiles, TurnRecord } from "@/lib/types";
import { useApi } from "@/lib/use-api";
import { PageBody, PageHeader } from "@/components/shell/page-header";
import { TurnsTable } from "@/components/supervisor/turns-table";
import { FIRST_AUDIO_TARGET_MS, ROUTE_TARGET_MS } from "@/components/trace/timings-waterfall";
import { BarList } from "@/components/ui/bar-list";
import { Button } from "@/components/ui/button";
import { ConfidenceBar, DEFAULT_THRESHOLDS } from "@/components/ui/confidence-bar";
import { Metric, MetricGrid } from "@/components/ui/metric";
import { Panel } from "@/components/ui/panel";
import { LangBadge, PathBadge, Pill, PolicyBadge, ScenarioTag } from "@/components/ui/pill";
import { EmptyState, ErrorState, LoadingState } from "@/components/ui/states";
import { Table, Td, Th } from "@/components/ui/table";

const loadSessions = () => api.sessions(50);

function pctl(p: Percentiles | undefined, k: "p50" | "p90"): string {
  if (!p || p.n === 0) return "—";
  return fmtNum(p[k]);
}

function latencyTone(v: number | undefined, target: number): "success" | "warning" | undefined {
  if (v === undefined) return undefined;
  return v <= target ? "success" : "warning";
}

export default function SupervisorPage() {
  const stats = useApi(api.stats, { interval: 5000 });
  const sessions = useApi(loadSessions, { interval: 10_000 });
  const config = useApi(api.config);
  const names = useScenarioNames();
  const thresholds = useMemo(
    () => ({ proceed: config.data?.policy.proceed_min ?? DEFAULT_THRESHOLDS.proceed, clarify: config.data?.policy.clarify_min ?? DEFAULT_THRESHOLDS.clarify }),
    [config.data],
  );

  const s = stats.data;
  const recent = useMemo(() => (s?.recent ?? []).slice().reverse(), [s]);
  const uncertain = useMemo(() => (s?.uncertain ?? []).slice().reverse().slice(0, 12), [s]);
  const scenarioItems = useMemo(
    () =>
      Object.entries(s?.by_scenario ?? {})
        .sort((a, b) => b[1] - a[1])
        .slice(0, 12)
        .map(([id, n]) => ({ key: id, label: <span><span className="font-mono tracking-normal text-foreground/85">{id}</span> <span className="text-muted-foreground">{names[id] ? `· ${truncate(names[id], 40)}` : ""}</span></span>, value: n })),
    [s, names],
  );
  const langItems = useMemo(
    () =>
      Object.entries(s?.by_language ?? {})
        .sort((a, b) => b[1] - a[1])
        .map(([l, n]) => ({ key: l, label: <LangBadge lang={l} />, value: n, tone: l === "mixed" ? ("primary" as const) : ("neutral" as const) })),
    [s],
  );

  const turns = s?.turns ?? 0;
  const fastShare = ratio(s?.by_path?.fast, turns);
  const agree = ratio(s?.fast_path_agree, s?.fast_path_checked);
  const route = s?.timings?.route;
  const firstAudio = s?.timings?.first_audio;

  return (
    <PageBody className="space-y-4">
      <PageHeader
        title="Supervisor"
        meta={stats.fetchedAt ? <span className="text-[11px] text-muted-foreground/55">auto-refresh 5 s</span> : undefined}
        actions={
          <Button size="sm" onClick={() => { stats.refresh(); sessions.refresh(); }} disabled={stats.refreshing}>
            <RefreshCw className={stats.refreshing ? "size-3.5 animate-spin" : "size-3.5"} /> Refresh
          </Button>
        }
      />

      {stats.error && !s ? (
        <ErrorState message={stats.error} onRetry={stats.refresh} />
      ) : !s ? (
        <div className="rounded-[15px] border border-border bg-card">
          <LoadingState rows={5} />
        </div>
      ) : (
        <>
          <MetricGrid cols={4}>
            <Metric label="Turns" value={fmtNum(s.turns)} foot={`${fmtNum(s.sessions)} sessions`} />
            <Metric label="Handoffs" value={fmtNum(s.handoffs)} unit={turns ? fmtPct(ratio(s.handoffs, turns)) : undefined} foot="transferred to a human queue" />
            <Metric
              label="Low-confidence share"
              value={turns ? fmtPct(ratio(s.low_confidence, turns)) : "—"}
              unit={`${fmtNum(s.low_confidence)} turns`}
              tone={turns && (ratio(s.low_confidence, turns) ?? 0) > 0.2 ? "warning" : undefined}
              foot={`below proceed threshold ${fmtPct(thresholds.proceed)}`}
            />
            <Metric
              label="Fast-path share"
              value={turns ? fmtPct(fastShare ?? 0) : "—"}
              unit={s.fast_path_checked ? `agree ${fmtPct(agree)}` : undefined}
              foot={s.fast_path_checked ? `${s.fast_path_agree}/${s.fast_path_checked} verified by the LLM shadow check` : "no shadow checks yet"}
            />
          </MetricGrid>

          <MetricGrid cols={4}>
            <Metric label="Route p50" value={pctl(route, "p50")} unit="ms" tone={latencyTone(route?.p50, ROUTE_TARGET_MS)} foot={`target ≤ ${ROUTE_TARGET_MS} ms · n=${route?.n ?? 0}`} />
            <Metric label="Route p90" value={pctl(route, "p90")} unit="ms" tone={latencyTone(route?.p90, ROUTE_TARGET_MS)} foot={route?.max ? `max ${fmtNum(route.max)} ms` : "no measurements"} />
            <Metric label="First audio p50" value={pctl(firstAudio, "p50")} unit="ms" tone={latencyTone(firstAudio?.p50, FIRST_AUDIO_TARGET_MS)} foot={`target ≤ ${FIRST_AUDIO_TARGET_MS} ms · n=${firstAudio?.n ?? 0}`} />
            <Metric label="First audio p90" value={pctl(firstAudio, "p90")} unit="ms" tone={latencyTone(firstAudio?.p90, FIRST_AUDIO_TARGET_MS)} foot={firstAudio?.max ? `max ${fmtNum(firstAudio.max)} ms` : "no measurements"} />
          </MetricGrid>

          {Object.keys(s.first_audio_by_path ?? {}).length > 0 && (
            <Panel title="First audio by path" bodyClassName="p-0">
              <Table>
                <thead>
                  <tr>
                    <Th>path</Th>
                    <Th className="text-right">n</Th>
                    <Th className="text-right">p50</Th>
                    <Th className="text-right">p90</Th>
                    <Th className="text-right">avg</Th>
                    <Th className="text-right">max</Th>
                  </tr>
                </thead>
                <tbody>
                  {Object.entries(s.first_audio_by_path ?? {}).map(([p, v]) => (
                    <tr key={p}>
                      <Td>
                        <PathBadge path={p} />
                      </Td>
                      <Td className="text-right tabular-nums">{v.n}</Td>
                      <Td className="text-right tabular-nums">{v.p50}</Td>
                      <Td className="text-right tabular-nums">{v.p90}</Td>
                      <Td className="text-right tabular-nums">{v.avg}</Td>
                      <Td className="text-right tabular-nums">{v.max}</Td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            </Panel>
          )}

          <div className="grid min-w-0 gap-4 xl:grid-cols-3">
            <Panel title="Recent turns" note={`${recent.length} of ${s.turns}`} className="xl:col-span-2" bodyClassName="p-0">
              <TurnsTable rows={recent} names={names} thresholds={thresholds} />
            </Panel>

            <div className="min-w-0 space-y-4">
              <Panel title="Uncertain cases" note={`${s.low_confidence} total`} bodyClassName="p-2">
                {uncertain.length === 0 ? (
                  <EmptyState title="Nothing uncertain" hint="Turns with confidence below the proceed threshold land here." className="py-6" />
                ) : (
                  <ul className="divide-y divide-border">
                    {uncertain.map((r: TurnRecord) => (
                      <li key={`${r.session_id}-${r.turn}`} className="px-2 py-2">
                        <Link href={`/supervisor/${encodeURIComponent(r.session_id)}`} className="block text-[12.5px] leading-snug text-foreground/90 hover:text-primary">
                          {truncate(r.transcript, 110)}
                        </Link>
                        <div className="mt-1 flex flex-wrap items-center gap-1.5">
                          {(r.scenarios ?? []).slice(0, 2).map((id) => (
                            <ScenarioTag key={id} id={id} name={names[id]} />
                          ))}
                          <ConfidenceBar value={r.confidence} thresholds={thresholds} width="w-10" />
                          <PolicyBadge action={r.policy_action} />
                          <span className="ml-auto font-mono text-[10.5px] tracking-normal text-muted-foreground/50">{shortId(r.session_id, 8)}</span>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </Panel>

              <Panel title="Scenario frequency" note={`${Object.keys(s.by_scenario ?? {}).length} distinct`}>
                {scenarioItems.length ? <BarList items={scenarioItems} /> : <EmptyState title="No scenarios yet" className="py-6" />}
              </Panel>

              <Panel title="Languages">
                {langItems.length ? <BarList items={langItems} /> : <EmptyState title="No turns yet" className="py-6" />}
              </Panel>

              <Panel title="Paths · policy">
                <div className="flex flex-wrap gap-1.5">
                  {Object.entries(s.by_path ?? {}).map(([p, n]) => (
                    <span key={p} className="inline-flex items-center gap-1">
                      <PathBadge path={p} />
                      <span className="text-[11px] tabular-nums text-muted-foreground">{n}</span>
                    </span>
                  ))}
                </div>
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {Object.entries(s.by_policy ?? {}).map(([p, n]) => (
                    <span key={p} className="inline-flex items-center gap-1">
                      <PolicyBadge action={p} />
                      <span className="text-[11px] tabular-nums text-muted-foreground">{n}</span>
                    </span>
                  ))}
                </div>
              </Panel>
            </div>
          </div>
        </>
      )}

      <Panel title="Sessions" note={sessions.data ? `${(sessions.data.sessions ?? []).length} shown` : undefined} bodyClassName="p-0">
        {sessions.error && !sessions.data ? (
          <ErrorState message={sessions.error} onRetry={sessions.refresh} className="m-4" />
        ) : !sessions.data ? (
          <LoadingState rows={4} />
        ) : (sessions.data.sessions ?? []).length === 0 ? (
          <EmptyState title="No sessions" hint="Open the Call page and start talking." />
        ) : (
          <Table>
            <thead>
              <tr>
                <Th>session</Th>
                <Th>last activity</Th>
                <Th>channel</Th>
                <Th className="text-right">turns</Th>
                <Th>lang</Th>
                <Th>first utterance</Th>
                <Th>client</Th>
                <Th>handoff</Th>
              </tr>
            </thead>
            <tbody>
              {(sessions.data.sessions ?? []).map((se) => (
                <tr key={se.id} className="hover:bg-foreground/[0.025]">
                  <Td>
                    <Link href={`/supervisor/${encodeURIComponent(se.id)}`} className="font-mono text-[11.5px] tracking-normal text-primary hover:underline">
                      {se.id}
                    </Link>
                  </Td>
                  <Td className="whitespace-nowrap tabular-nums text-muted-foreground">{fmtDateTime(se.last_at)}</Td>
                  <Td>
                    <Pill>{se.channel || "—"}</Pill>
                  </Td>
                  <Td className="text-right tabular-nums">{se.turns}</Td>
                  <Td>
                    <LangBadge lang={se.language} />
                  </Td>
                  <Td className="max-w-[360px]">
                    <span className="block truncate" title={se.title}>
                      {se.title || <span className="text-muted-foreground/40">—</span>}
                    </span>
                  </Td>
                  <Td className="font-mono text-[11.5px] tracking-normal">{se.client_id || <span className="text-muted-foreground/40">—</span>}</Td>
                  <Td>{se.handoff ? <Pill tone="info">handoff</Pill> : <span className="text-muted-foreground/40">—</span>}</Td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Panel>
    </PageBody>
  );
}
