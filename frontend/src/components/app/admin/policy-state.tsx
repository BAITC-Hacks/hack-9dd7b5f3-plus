"use client";
/** Policy verdict, dialog state, executed actions. */
import { Badge } from "@/components/ui/badge";
import { scenarioLabel } from "@/lib/catalog";
import { CONFIDENCE_CLARIFY, CONFIDENCE_RUN, type DialogState } from "@/lib/contract";
import { Dash, fmtMs, KV, ModeBadge, MonoChip, PolicyBadge, Section, type TurnView } from "./shared";

function ScenarioChips({ ids }: { ids: string[] }) {
  if (!ids.length) return <Dash />;
  return (
    <span className="inline-flex flex-wrap justify-end gap-1">
      {ids.map((id) => (
        <MonoChip key={id}>{id} · {scenarioLabel(id, "ru")}</MonoChip>
      ))}
    </span>
  );
}

export function PolicyCard({ view }: { view: TurnView }) {
  const v = view.verdict;
  return (
    <Section index="04 / 07" title="Политика" right={<PolicyBadge action={v?.action} />}>
      {v ? (
        <>
          <p className="text-sm leading-relaxed">{v.reason || <Dash />}</p>
          <div className="mt-3">
            <KV k="сценарий">{v.scenario_id ? <span className="font-mono text-xs">{v.scenario_id} · <span className="font-sans text-sm">{scenarioLabel(v.scenario_id, "ru")}</span></span> : <Dash />}</KV>
            {v.queue && <KV k="очередь"><span className="font-mono text-xs">{v.queue}</span></KV>}
            <KV k="стек"><ScenarioChips ids={v.stack} /></KV>
            <KV k="low_conf_streak"><span className="num">{v.low_conf_streak}</span></KV>
          </div>
        </>
      ) : (
        <div className="text-sm text-muted-foreground">Политика ещё не применялась — <Dash /></div>
      )}
      <div className="hairline-t mt-3 pt-2 font-mono text-[11px] tracking-[.06em] text-muted-foreground">
        ≥ {CONFIDENCE_RUN.toFixed(2)} запуск · {CONFIDENCE_CLARIFY.toFixed(2)}–{CONFIDENCE_RUN.toFixed(2)} уточнение · &lt; {CONFIDENCE_CLARIFY.toFixed(2)} ×2 → оператор
      </div>
    </Section>
  );
}

export function DialogStateCard({ dialog }: { dialog: DialogState }) {
  const slots = Object.entries(dialog.slots ?? {}).filter(([k]) => !k.startsWith("__"));
  const awaiting = dialog.awaiting
    ? dialog.awaiting.kind === "slot"
      ? `слот · ${dialog.awaiting.slot}`
      : `подтверждение · ${dialog.awaiting.action}`
    : null;
  return (
    <Section index="05 / 07" title="Состояние диалога" right={<span className="marker">ход {dialog.turn}{dialog.ended ? " · завершён" : ""}</span>}>
      <KV k="язык ответа"><Badge variant={dialog.language === "kk" ? "success" : "secondary"} size="sm" className="font-mono tracking-[.08em]">{dialog.language.toUpperCase()}</Badge></KV>
      <KV k="клиент">{dialog.client_name ? <>{dialog.client_name} <span className="font-mono text-xs text-muted-foreground">{dialog.client_id}</span></> : <Dash />}</KV>
      <KV k="активный сценарий">{dialog.active_scenario ? <span className="font-mono text-xs">{dialog.active_scenario} · <span className="font-sans text-sm">{scenarioLabel(dialog.active_scenario, "ru")}</span></span> : <Dash />}</KV>
      <KV k="стек"><ScenarioChips ids={dialog.stack} /></KV>
      <KV k="ожидание">{awaiting ? <Badge variant="warning" size="sm" className="font-mono tracking-[.06em]">{awaiting}</Badge> : <Dash />}</KV>
      <div className="pt-2">
        <div className="marker mb-1">слоты · {slots.length}</div>
        {slots.length === 0 ? (
          <div className="text-sm text-muted-foreground">пусто</div>
        ) : (
          <ul className="divide-y divide-border">
            {slots.map(([k, val]) => (
              <li key={k} className="flex items-baseline justify-between gap-3 py-1.5">
                <span className="font-mono text-xs text-muted-foreground">{k}</span>
                <span className="min-w-0 truncate text-right font-mono text-xs">{typeof val === "string" || typeof val === "number" || typeof val === "boolean" ? String(val) : JSON.stringify(val)}</span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </Section>
  );
}

function summarize(r: Record<string, unknown> | undefined): string {
  if (!r) return "";
  return Object.entries(r)
    .slice(0, 2)
    .map(([k, v]) => `${k}: ${typeof v === "object" && v !== null ? JSON.stringify(v).slice(0, 40) : String(v)}`)
    .join(" · ");
}

export function ActionsCard({ view }: { view: TurnView }) {
  const a = view.actions;
  return (
    <Section index="06 / 07" title="Действия" right={<span className="marker">{a.length} вызов(ов)</span>} bodyClassName="p-0">
      {a.length === 0 ? (
        <div className="p-4 text-sm text-muted-foreground">Инструменты не вызывались — <Dash /></div>
      ) : (
        <ol className="divide-y divide-border">
          {a.map((c, i) => (
            <li key={i} className="grid grid-cols-[20px_minmax(0,1fr)_auto_56px] items-center gap-x-3 px-4 py-2.5">
              <span className="marker">{String(i + 1).padStart(2, "0")}</span>
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="font-mono text-xs">{c.name}</span>
                  <ModeBadge mode={c.mode} />
                </div>
                {c.error ? (
                  <div className="mt-1 flex items-center gap-2 text-xs">
                    <Badge variant="error" size="sm" className="font-mono">{c.error.code}</Badge>
                    <span className="truncate text-destructive-foreground">{c.error.message}</span>
                  </div>
                ) : (
                  <div className="mt-1 truncate font-mono text-[11px] text-muted-foreground">{summarize(c.result) || (c.input ? `in · ${summarize(c.input)}` : "—")}</div>
                )}
              </div>
              <span />
              <span className="num text-xs">{typeof c.ms === "number" ? `${fmtMs(c.ms)}` : "—"}</span>
            </li>
          ))}
        </ol>
      )}
    </Section>
  );
}
