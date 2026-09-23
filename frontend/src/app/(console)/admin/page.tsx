"use client";
/**
 * /admin — Консоль супервизора («под капотом»).
 * Every turn is traced: live candidates → decision → policy → actions → latency waterfall.
 */
import { useMemo, useState } from "react";
import { ConversationPanel } from "@/components/app/conversation-panel";
import { LiveCandidates } from "@/components/app/admin/candidates";
import { DecisionCard } from "@/components/app/admin/decision";
import { LatencyWaterfall, PipelineStepper } from "@/components/app/admin/pipeline";
import { ActionsCard, DialogStateCard, PolicyCard } from "@/components/app/admin/policy-state";
import { MetricCard, readmeTrace, toTurnView } from "@/components/app/admin/shared";
import { TurnLog } from "@/components/app/admin/turn-log";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { computeStats } from "@/lib/api";
import { useConversation } from "@/lib/store";

export default function AdminPage() {
  const s = useConversation();
  const [showJson, setShowJson] = useState(false);

  const stats = useMemo(() => computeStats(s.traces, 1), [s.traces]);
  const lastTrace = s.traces[s.traces.length - 1];
  const view = useMemo(() => toTurnView(s.live, lastTrace), [s.live, lastTrace]);
  const json = useMemo(() => (lastTrace ? JSON.stringify(readmeTrace(lastTrace), null, 2) : null), [lastTrace]);

  return (
    <div className="mx-auto w-full max-w-[1248px] space-y-6 px-4 py-6 sm:px-8">
      {/* A. header */}
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="h-display text-2xl sm:text-3xl">Консоль супервизора</h1>
          <div className="marker mt-1.5">
            трассировка каждой реплики · {s.mode} · {s.traces.length} {plural(s.traces.length)}
            {s.live && s.live.stage !== "done" && <span className="text-brand"> · live</span>}
          </div>
        </div>
        <Button variant="secondary" size="sm" onClick={() => setShowJson((v) => !v)} disabled={!json}>
          {showJson ? "Скрыть JSON" : "JSON трассировки"}
        </Button>
      </header>

      {showJson && json && (
        <Card className="rounded-xl shadow-none before:hidden">
          <div className="hairline-b flex items-center justify-between px-4 py-2.5">
            <span className="marker marker-dot">trace · ход {lastTrace?.turn} · формат README</span>
            <span className="marker">{json.length} байт</span>
          </div>
          <pre className="max-h-[420px] overflow-auto p-4 font-mono text-xs leading-relaxed">{json}</pre>
        </Card>
      )}

      {/* B. metrics */}
      <section className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-5">
        <MetricCard label="ходов" value={String(stats.turns)} />
        <MetricCard label="средняя уверенность" value={stats.turns ? `${Math.round(stats.avg_confidence * 100)}` : "—"} unit={stats.turns ? "%" : undefined} />
        <MetricCard label="низкая уверенность" value={String(stats.low_confidence_turns)} description="уточнение или передача" />
        <MetricCard label="передач оператору" value={String(stats.handoffs)} />
        <MetricCard label="медиана задержки" value={stats.turns ? stats.median_total_ms.toLocaleString("ru-RU") : "—"} unit={stats.turns ? "ms" : undefined} description="от конца реплики до первого звука; ориентир 1 500 ms" />
      </section>

      {/* C. two columns */}
      <section className="grid gap-6 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
        <Card className="h-[640px] overflow-hidden rounded-xl shadow-none before:hidden">
          <div className="hairline-b flex items-center justify-between px-4 py-2.5">
            <span className="marker marker-dot">Разговор</span>
            <span className="marker">{s.status}{s.sttLang === "kk-KZ" ? " · kk" : " · ru"}</span>
          </div>
          <ConversationPanel compact className="min-h-0 flex-1" />
        </Card>

        <div className="space-y-4">
          <div className="marker">Под капотом · {view.source === "live" ? "текущий ход" : view.source === "trace" ? "последний ход" : "ожидание реплики"}</div>
          <PipelineStepper view={view} />
          <LiveCandidates view={view} />
          <DecisionCard view={view} />
          <PolicyCard view={view} />
          <DialogStateCard dialog={s.dialog} />
          <ActionsCard view={view} />
          <LatencyWaterfall latency={view.latency} />
          {(view.error || s.error) && (
            <Card className="rounded-xl border-destructive/40 shadow-none before:hidden">
              <div className="p-4 text-sm">
                <div className="marker mb-1 text-destructive-foreground">ошибка</div>
                {view.error ?? s.error}
              </div>
            </Card>
          )}
        </div>
      </section>

      {/* D. log */}
      <TurnLog traces={s.traces} />
    </div>
  );
}

function plural(n: number): string {
  const m10 = n % 10, m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return "ход";
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return "хода";
  return "ходов";
}
