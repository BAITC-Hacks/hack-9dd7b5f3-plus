"use client";

import Link from "next/link";
import { useCallback, useMemo } from "react";
import { ArrowLeft, RefreshCw } from "lucide-react";
import { api } from "@/lib/api";
import { useScenarioNames } from "@/lib/catalog-cache";
import { fmtDateTime, langLabel } from "@/lib/format";
import type { TraceView, TurnRecord } from "@/lib/types";
import { useApi } from "@/lib/use-api";
import { PageBody, PageHeader } from "@/components/shell/page-header";
import { TurnsTable } from "@/components/supervisor/turns-table";
import { TracePanel } from "@/components/trace/trace-panel";
import { Button } from "@/components/ui/button";
import { DEFAULT_THRESHOLDS } from "@/components/ui/confidence-bar";
import { KV, valueText } from "@/components/ui/kv";
import { Panel } from "@/components/ui/panel";
import { LangBadge, Pill } from "@/components/ui/pill";
import { EmptyState, ErrorState, LoadingState } from "@/components/ui/states";

function toView(r: TurnRecord): TraceView | null {
  const tr = r.trace;
  if (!tr) return null;
  return {
    turn: r.turn,
    transcript: tr.input?.transcript ?? r.transcript,
    source: tr.input?.source,
    reply: tr.reply?.text ?? "",
    replyDone: true,
    trace: tr,
    done: true,
    at: Date.parse(r.at),
  };
}

export function SessionView({ id }: { id: string }) {
  const loader = useCallback(() => api.session(id), [id]);
  const { data, error, loading, refresh, refreshing } = useApi(loader);
  const config = useApi(api.config);
  const names = useScenarioNames();
  const thresholds = useMemo(
    () => ({ proceed: config.data?.policy.proceed_min ?? DEFAULT_THRESHOLDS.proceed, clarify: config.data?.policy.clarify_min ?? DEFAULT_THRESHOLDS.clarify }),
    [config.data],
  );
  const turns = useMemo(() => (data?.turns ?? []).slice().sort((a, b) => a.turn - b.turn), [data]);
  const state = data?.state;

  return (
    <PageBody className="space-y-4">
      <PageHeader
        title="Session"
        meta={
          <span className="flex items-center gap-1.5">
            <Pill mono>{id}</Pill>
            {data?.session.language && <LangBadge lang={data.session.language} />}
            {data?.session.handoff && <Pill tone="info">handoff</Pill>}
          </span>
        }
        actions={
          <>
            <Button size="sm" variant="ghost" onClick={refresh} disabled={refreshing}>
              <RefreshCw className={refreshing ? "size-3.5 animate-spin" : "size-3.5"} /> Refresh
            </Button>
            <Link href="/supervisor" className="inline-flex h-8 items-center gap-1.5 rounded-[10px] border border-border px-3 text-[12.5px] font-medium text-foreground/80 hover:bg-foreground/[0.05]">
              <ArrowLeft className="size-3.5" /> Supervisor
            </Link>
          </>
        }
      />

      {error && !data ? (
        <ErrorState message={error} onRetry={refresh} />
      ) : loading && !data ? (
        <div className="rounded-[15px] border border-border bg-card">
          <LoadingState rows={5} />
        </div>
      ) : data ? (
        <>
          <div className="grid gap-4 lg:grid-cols-2">
            <Panel title="Session">
              <KV
                mono={false}
                items={[
                  ["started", fmtDateTime(data.session.created_at)],
                  ["last activity", fmtDateTime(data.session.last_at)],
                  ["channel", data.session.channel || "—"],
                  ["turns", String(data.session.turns)],
                  ["language", langLabel(data.session.language)],
                  ["client", data.session.client_id || "not identified"],
                ]}
              />
            </Panel>
            <Panel title="Dialogue state" note={state ? "live" : "session closed"}>
              {state ? (
                <KV
                  mono={false}
                  items={[
                    ["active scenario", state.active_scenario ? `${state.active_scenario} — ${names[state.active_scenario] ?? ""}` : "—"],
                    ["stack", state.stack && state.stack.length ? state.stack.join(" → ") : "—"],
                    ["client", state.client_id ? `${state.client_name ?? ""} (${state.client_id})` : "—"],
                    ["pending confirmation", state.pending_confirmation ? state.pending_confirmation.name : "—"],
                    ["clarify count", String(state.clarify_count)],
                    ["slots", Object.keys(state.slots ?? {}).length ? Object.entries(state.slots ?? {}).map(([k, v]) => `${k}=${valueText(v)}`).join(", ") : "—"],
                    ["closed", state.closed ? "yes" : "no"],
                  ]}
                />
              ) : (
                <p className="text-[12px] text-muted-foreground">The live state is only available while the session is open in the engine (restored sessions show turns only).</p>
              )}
            </Panel>
          </div>

          <Panel title="Turns" note={`${turns.length}`} bodyClassName="p-0">
            <TurnsTable rows={turns} names={names} thresholds={thresholds} showSession={false} emptyHint="This session has no recorded turns yet." />
          </Panel>

          {turns.length > 0 && (
            <div className="space-y-4">
              {turns.map((r) => {
                const view = toView(r);
                return (
                  <Panel key={r.turn} title={`Turn ${r.turn}`} note={fmtDateTime(r.at)} id={`turn-${r.turn}`}>
                    {view ? (
                      <TracePanel view={view} names={names} thresholds={thresholds} compact showReply />
                    ) : (
                      <EmptyState title="Trace not stored" hint="This record has no trace payload." className="py-4" />
                    )}
                  </Panel>
                );
              })}
            </div>
          )}
        </>
      ) : null}
    </PageBody>
  );
}
