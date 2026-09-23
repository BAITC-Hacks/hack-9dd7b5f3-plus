"use client";

import { useCallback, useMemo, useState } from "react";
import { Check, Play, X } from "lucide-react";
import { api, ApiError, errorMessage } from "@/lib/api";
import { cn } from "@/lib/cn";
import { useScenarioNames } from "@/lib/catalog-cache";
import { fmtDateTime, fmtDuration, fmtNum, fmtPct } from "@/lib/format";
import type { EvalLast, EvalReport, EvalRow } from "@/lib/types";
import { useApi } from "@/lib/use-api";
import { PageBody, PageHeader } from "@/components/shell/page-header";
import { Button } from "@/components/ui/button";
import { ConfidenceBar } from "@/components/ui/confidence-bar";
import { Metric, MetricGrid } from "@/components/ui/metric";
import { Panel } from "@/components/ui/panel";
import { LangBadge, PathBadge, Pill, ScenarioTag } from "@/components/ui/pill";
import { EmptyState, ErrorState, LoadingState } from "@/components/ui/states";
import { Table, Td, Th } from "@/components/ui/table";

function groupOrder(keys: string[]): string[] {
  const langs = keys.filter((k) => k.startsWith("lang=")).sort();
  const types = keys.filter((k) => k.startsWith("type=")).sort();
  const rest = keys.filter((k) => k !== "all" && !k.startsWith("lang=") && !k.startsWith("type=")).sort();
  return [...(keys.includes("all") ? ["all"] : []), ...langs, ...types, ...rest];
}

function OkIcon({ ok }: { ok: boolean }) {
  return ok ? <Check className="size-3.5 text-success" aria-label="ok" /> : <X className="size-3.5 text-destructive" aria-label="miss" />;
}

function Report({ report, names }: { report: EvalReport; names: Record<string, string> }) {
  const [errorsOnly, setErrorsOnly] = useState(false);
  const [q, setQ] = useState("");
  const [lang, setLang] = useState("all");
  const [type, setType] = useState("all");
  const rows = useMemo(() => report.rows ?? [], [report]);
  const langs = useMemo(() => Array.from(new Set(rows.map((r) => r.lang))).sort(), [rows]);
  const types = useMemo(() => Array.from(new Set(rows.map((r) => r.type))).sort(), [rows]);
  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return rows.filter((r: EvalRow) => {
      if (errorsOnly && r.primary_ok && r.full_ok && !r.error) return false;
      if (lang !== "all" && r.lang !== lang) return false;
      if (type !== "all" && r.type !== type) return false;
      if (needle) {
        const hay = `${r.id} ${r.text} ${(r.expected ?? []).join(" ")} ${(r.got ?? []).join(" ")} ${r.reason ?? ""}`.toLowerCase();
        if (!hay.includes(needle)) return false;
      }
      return true;
    });
  }, [rows, errorsOnly, q, lang, type]);
  const all = report.groups?.all;
  const misses = rows.filter((r) => !r.primary_ok).length;

  return (
    <div className="space-y-4">
      <MetricGrid cols={4}>
        <Metric label="Utterances" value={fmtNum(report.n)} foot={`router ${report.router} · ${fmtDateTime(report.ran_at)}`} />
        <Metric label="Primary accuracy" value={fmtPct(all?.primary_acc, 1)} unit={`${all?.primary ?? 0}/${all?.n ?? 0}`} tone={(all?.primary_acc ?? 0) >= 0.8 ? "success" : "warning"} foot={`${misses} misses`} />
        <Metric label="Full match" value={fmtPct(all?.full_match, 1)} unit={`${all?.full ?? 0}/${all?.n ?? 0}`} foot="all expected scenarios, no extras" />
        <Metric label="Intent recall" value={fmtPct(report.intent_recall, 1)} foot="multi-intent utterances" />
      </MetricGrid>
      <MetricGrid cols={4}>
        <Metric label="Route latency p50" value={fmtNum(report.latency_p50_ms)} unit="ms" />
        <Metric label="Route latency p90" value={fmtNum(report.latency_p90_ms)} unit="ms" />
        <Metric label="Route latency max" value={fmtNum(report.latency_max_ms)} unit="ms" />
        <Metric label="Run duration" value={fmtDuration(report.duration_ms)} foot={Object.entries(report.by_path ?? {}).map(([p, n]) => `${p} ${n}`).join(" · ") || undefined} />
      </MetricGrid>

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_280px]">
        <Panel title="Groups" bodyClassName="p-0">
          <Table>
            <thead>
              <tr>
                <Th>group</Th>
                <Th className="text-right">n</Th>
                <Th>primary_acc</Th>
                <Th>full_match</Th>
              </tr>
            </thead>
            <tbody>
              {groupOrder(Object.keys(report.groups ?? {})).map((k) => {
                const g = report.groups[k];
                return (
                  <tr key={k} className={k === "all" ? "bg-foreground/[0.03]" : undefined}>
                    <Td className="font-mono text-[11.5px] tracking-normal">{k}</Td>
                    <Td className="text-right tabular-nums">{g.n}</Td>
                    <Td>
                      <span className="inline-flex items-center gap-2">
                        <span className="h-1.5 w-24 overflow-hidden rounded-full bg-primary/[0.15]">
                          <span className="block h-full rounded-full bg-primary" style={{ width: `${Math.round(g.primary_acc * 100)}%` }} />
                        </span>
                        <span className="tabular-nums">{fmtPct(g.primary_acc, 1)}</span>
                      </span>
                    </Td>
                    <Td>
                      <span className="inline-flex items-center gap-2">
                        <span className="h-1.5 w-24 overflow-hidden rounded-full bg-primary/[0.15]">
                          <span className="block h-full rounded-full bg-info" style={{ width: `${Math.round(g.full_match * 100)}%` }} />
                        </span>
                        <span className="tabular-nums">{fmtPct(g.full_match, 1)}</span>
                      </span>
                    </Td>
                  </tr>
                );
              })}
            </tbody>
          </Table>
        </Panel>
        <Panel title="Paths">
          <div className="flex flex-wrap gap-2">
            {Object.entries(report.by_path ?? {}).map(([p, n]) => (
              <span key={p} className="inline-flex items-center gap-1.5">
                <PathBadge path={p} />
                <span className="text-[12px] tabular-nums text-muted-foreground">{n}</span>
              </span>
            ))}
            {Object.keys(report.by_path ?? {}).length === 0 && <span className="text-[12px] text-muted-foreground/45">—</span>}
          </div>
          <p className="mt-3 text-[11.5px] leading-relaxed text-muted-foreground">
            Same metrics as the kit&apos;s <span className="font-mono tracking-normal">evaluate.py</span>: primary = first scenario matches, full = exact set match, recall over multi-intent utterances.
          </p>
        </Panel>
      </div>

      <Panel
        title="Utterances"
        note={`${filtered.length} of ${rows.length}`}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Search…"
              aria-label="Search utterances"
              className="h-7 w-40 rounded-[8px] border border-input bg-background/60 px-2 text-[12px] outline-none focus-visible:border-primary/60"
            />
            <select value={lang} onChange={(e) => setLang(e.target.value)} aria-label="Language" className="h-7 rounded-[8px] border border-border bg-muted px-1.5 text-[11.5px]">
              <option value="all">all langs</option>
              {langs.map((l) => (
                <option key={l} value={l}>
                  {l}
                </option>
              ))}
            </select>
            <select value={type} onChange={(e) => setType(e.target.value)} aria-label="Type" className="h-7 rounded-[8px] border border-border bg-muted px-1.5 text-[11.5px]">
              <option value="all">all types</option>
              {types.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
            <Button size="sm" variant={errorsOnly ? "danger" : "ghost"} onClick={() => setErrorsOnly((v) => !v)} aria-pressed={errorsOnly}>
              Errors only
            </Button>
          </div>
        }
        bodyClassName="p-0"
      >
        {filtered.length === 0 ? (
          <EmptyState title="No rows match" hint="Loosen the filter." />
        ) : (
          <Table>
            <thead>
              <tr>
                <Th>id</Th>
                <Th>text</Th>
                <Th>lang</Th>
                <Th>type</Th>
                <Th>expected</Th>
                <Th>got</Th>
                <Th className="text-center">primary</Th>
                <Th className="text-center">full</Th>
                <Th>confidence</Th>
                <Th>path</Th>
                <Th className="text-right">route ms</Th>
                <Th>reason</Th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((r) => (
                <tr key={r.id} className={cn("hover:bg-foreground/[0.025]", !r.primary_ok && "bg-destructive/[0.03]")}>
                  <Td className="font-mono text-[11.5px] tracking-normal text-muted-foreground">{r.id}</Td>
                  <Td className="min-w-[260px] max-w-[420px]">
                    <span className="block text-[12.5px] leading-snug">{r.text}</span>
                    {r.error && <span className="text-[11px] text-destructive">{r.error}</span>}
                  </Td>
                  <Td>
                    <LangBadge lang={r.lang} />
                  </Td>
                  <Td>
                    <Pill>{r.type}</Pill>
                  </Td>
                  <Td>
                    <span className="flex flex-wrap gap-1">
                      {(r.expected ?? []).map((s) => (
                        <ScenarioTag key={s} id={s} name={names[s]} />
                      ))}
                    </span>
                  </Td>
                  <Td>
                    <span className="flex flex-wrap gap-1">
                      {(r.got ?? []).length ? (r.got ?? []).map((s) => <ScenarioTag key={s} id={s} name={names[s]} />) : <span className="text-muted-foreground/40">—</span>}
                    </span>
                  </Td>
                  <Td className="text-center">
                    <OkIcon ok={r.primary_ok} />
                  </Td>
                  <Td className="text-center">
                    <OkIcon ok={r.full_ok} />
                  </Td>
                  <Td>
                    <ConfidenceBar value={r.confidence} width="w-12" />
                  </Td>
                  <Td>
                    <PathBadge path={r.path} />
                  </Td>
                  <Td className="text-right tabular-nums">{r.route_ms}</Td>
                  <Td className="max-w-[300px] text-[11.5px] text-muted-foreground">
                    <span className="block truncate" title={r.reason}>
                      {r.reason || "—"}
                    </span>
                  </Td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Panel>
    </div>
  );
}

export default function EvalPage() {
  const [runState, setRunState] = useState<{ running: boolean; error: string | null }>({ running: false, error: null });
  const ownRun = runState.running;
  // Poll every 2 s while our POST is in flight or the server reports a run (started here or elsewhere).
  const interval = useCallback((d: EvalLast | null) => (ownRun || d?.running ? 2000 : 0), [ownRun]);
  const last = useApi(api.evalLast, { interval });
  const serverRunning = !!last.data?.running;
  const running = ownRun || serverRunning;
  const report = last.data?.report ?? null;
  const progress = last.data?.progress ?? [0, 0];
  const names = useScenarioNames();

  const run = async () => {
    setRunState({ running: true, error: null });
    try {
      await api.evalRun({});
      setRunState({ running: false, error: null });
    } catch (e) {
      const msg = e instanceof ApiError && e.status === 409 ? "An evaluation is already running — showing its progress" : errorMessage(e);
      setRunState({ running: false, error: msg });
    } finally {
      last.refresh();
    }
  };

  const pct = progress[1] > 0 ? Math.round((progress[0] / progress[1]) * 100) : 0;

  return (
    <PageBody className="space-y-4">
      <PageHeader
        title="Eval"
        meta={report ? <span className="text-[11px] text-muted-foreground/55">last run {fmtDateTime(report.ran_at)}</span> : undefined}
        actions={
          <Button variant="primary" size="lg" onClick={() => void run()} disabled={running}>
            <Play className="size-3.5" /> {running ? "Running…" : "Run evaluation"}
          </Button>
        }
      />

      {running && (
        <Panel title="Running the dev set" note={progress[1] ? `${progress[0]} / ${progress[1]}` : "starting…"}>
          <div className="h-2 overflow-hidden rounded-full bg-primary/[0.15]">
            <div className="h-full rounded-full bg-primary transition-[width] duration-500" style={{ width: `${Math.max(3, pct)}%` }} />
          </div>
          <p className="mt-2 text-[11.5px] text-muted-foreground">Routes every utterance of data/dev_utterances.json through the router. With a real LLM this takes up to ~2 minutes; the mock router finishes in about a second.</p>
        </Panel>
      )}

      {runState.error && <ErrorState message={runState.error} onRetry={() => void run()} />}

      {last.error && !last.data ? (
        <ErrorState message={last.error} onRetry={last.refresh} />
      ) : last.loading && !last.data ? (
        <div className="rounded-[15px] border border-border bg-card">
          <LoadingState rows={4} />
        </div>
      ) : report ? (
        <Report report={report} names={names} />
      ) : (
        !running && (
          <div className="rounded-[15px] border border-border bg-card">
            <EmptyState
              title="No evaluation yet"
              hint="Run the labeled dev set to get primary accuracy, full match, multi-intent recall and route latency per language and utterance type."
              action={
                <Button variant="primary" onClick={() => void run()}>
                  <Play className="size-3.5" /> Run evaluation
                </Button>
              }
            />
          </div>
        )
      )}
    </PageBody>
  );
}
