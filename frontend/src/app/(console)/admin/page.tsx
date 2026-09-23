"use client";
/** /admin — supervisor console: what the robot understood, what it did, how fast, how good. */
import { useMemo, useState } from "react";
import { ConversationPanel } from "@/components/app/conversation-panel";
import { DecisionCard } from "@/components/app/admin/decision";
import { DialogCard } from "@/components/app/admin/dialog";
import { QualityCard } from "@/components/app/admin/quality";
import { fmtMs, Metric, readmeTrace, toTurnView } from "@/components/app/admin/shared";
import { SpeedCard } from "@/components/app/admin/speed";
import { TurnLog } from "@/components/app/admin/turn-log";
import { Button } from "@/components/ui/button";
import { computeStats } from "@/lib/api";
import { useConversation } from "@/lib/store";

export default function AdminPage() {
  const s = useConversation();
  const [showJson, setShowJson] = useState(false);

  const stats = useMemo(() => computeStats(s.traces, 1), [s.traces]);
  const lastTrace = s.traces[s.traces.length - 1];
  const view = useMemo(() => toTurnView(s.live, lastTrace), [s.live, lastTrace]);
  const json = useMemo(() => (lastTrace ? JSON.stringify(readmeTrace(lastTrace), null, 2) : null), [lastTrace]);
  const live = !!s.live && s.live.stage !== "done";

  return (
    <div className="mx-auto w-full max-w-[1200px] space-y-5 px-4 py-6 sm:px-8">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-medium">Консоль супервизора</h1>
          <p className="mt-0.5 text-sm text-muted-foreground">Каждая реплика: что понял робот, почему, что сделал и за сколько.</p>
        </div>
        <Button variant="secondary" size="sm" onClick={() => setShowJson((v) => !v)} disabled={!json}>{showJson ? "Скрыть JSON" : "JSON последней реплики"}</Button>
      </header>

      {showJson && json && <pre className="max-h-[380px] overflow-auto rounded-xl border border-border bg-card p-4 font-mono text-xs leading-relaxed">{json}</pre>}

      <section className="grid grid-cols-2 gap-3 lg:grid-cols-5">
        <Metric label="Реплик" value={String(stats.turns)} />
        <Metric label="Средняя уверенность" value={stats.turns ? `${Math.round(stats.avg_confidence * 100)}%` : "—"} />
        <Metric label="Переспросил" value={String(stats.clarifications)} hint="уверенность ниже 75%" warn={stats.clarifications > 0} />
        <Metric label="Передал оператору" value={String(stats.handoffs)} />
        <Metric label="Время ответа, медиана" value={stats.turns ? fmtMs(stats.median_total_ms) : "—"} unit={stats.turns ? "мс" : undefined} hint="ориентир 1 500 мс" warn={stats.turns > 0 && stats.median_total_ms > 1500} />
      </section>

      <section className="grid gap-5 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
        <div className="flex h-[720px] flex-col overflow-hidden rounded-xl border border-border bg-background">
          <div className="flex items-center justify-between border-b border-border px-4 py-2.5 text-sm">
            <span className="font-medium">Разговор</span>
            <span className="text-xs text-muted-foreground">{live ? "обрабатываю" : s.status === "listening" ? "слушаю" : s.status === "speaking" ? "отвечаю" : "готов"}</span>
          </div>
          <ConversationPanel compact className="min-h-0 flex-1" />
        </div>
        <div className="space-y-4">
          <DecisionCard view={view} />
          <DialogCard view={view} dialog={s.dialog} />
          <SpeedCard latency={view.latency} live={live} />
          {(view.error || s.error) && <div className="rounded-xl border border-destructive/40 bg-card p-4 text-sm text-destructive-foreground">{view.error ?? s.error}</div>}
        </div>
      </section>

      <QualityCard mode={s.mode} />
      <TurnLog traces={s.traces} />
    </div>
  );
}
