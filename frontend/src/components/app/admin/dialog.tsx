"use client";
import { useUiLanguage, translate as t } from "@/lib/ui-language";
/** «Ход диалога»: what the robot did, whom it talks to, what it waits for, which actions ran. */
import { Badge } from "@/components/ui/badge";
import type { DialogState } from "@/lib/contract";
import { scenarioLabel } from "@/lib/catalog";
import { ACTION_LABEL, Dash, MODE_LABEL, PolicyBadge, Row, Section, SLOT_LABEL, type TurnView } from "./shared";

function fmtVal(v: unknown): string {
  if (Array.isArray(v)) return v.join(", ");
  if (typeof v === "boolean") return v ? "да" : "нет";
  return String(v);
}

export function DialogCard({ view, dialog }: { view: TurnView; dialog: DialogState }) {
  const language = useUiLanguage();
  const v = view.verdict;
  const slots = Object.entries(dialog.slots).filter(([k]) => !k.startsWith("__"));
  const waiting = dialog.awaiting
    ? dialog.awaiting.kind === "slot"
      ? `${SLOT_LABEL[dialog.awaiting.slot] ?? dialog.awaiting.slot}`
      : `подтверждение «да» — ${ACTION_LABEL[dialog.awaiting.action] ?? dialog.awaiting.action}`
    : null;

  return (
    <Section title={t("Ход диалога")} right={<PolicyBadge action={v?.action} />}>
      {v?.reason && <div className="mb-2 text-xs text-muted-foreground">{v.reason}</div>}
      <Row k={t("Клиент")}>{dialog.client_name ? `${dialog.client_name} · ${dialog.client_id}` : <span className="text-muted-foreground">{t("не определён")}</span>}</Row>
      <Row k={t("Язык ответа")}>{dialog.language === "kk" ? t("казахский") : t("русский")}</Row>
      <Row k={t("Сейчас")}>{dialog.active_scenario ? scenarioLabel(dialog.active_scenario, language) : <Dash />}</Row>
      <Row k={t("Отложено")}>{dialog.stack.length ? dialog.stack.map((s) => scenarioLabel(s, language)).join(" → ") : <Dash />}</Row>
      <Row k={t("Ждёт от клиента")}>{waiting ?? <Dash />}</Row>

      <div className="mt-3 text-xs text-muted-foreground">{t("Что уже известно")}</div>
      {slots.length === 0 ? (
        <div className="py-1 text-sm text-muted-foreground">{t("пока ничего")}</div>
      ) : (
        <div className="mt-1 flex flex-wrap gap-1.5">
          {slots.map(([k, val]) => (
            <Badge key={k} variant="secondary" size="default" className="font-normal">
              <span className="text-muted-foreground">{SLOT_LABEL[k] ?? k}:</span>&nbsp;<span className="font-mono text-xs">{fmtVal(val)}</span>
            </Badge>
          ))}
        </div>
      )}

      <div className="mt-3 text-xs text-muted-foreground">{t("Действия в системе")}</div>
      {view.actions.length === 0 ? (
        <div className="py-1 text-sm text-muted-foreground">{t("в этом ходе не требовались")}</div>
      ) : (
        <ul className="mt-1 space-y-1">
          {view.actions.map((a, i) => (
            <li key={i} className="flex items-center justify-between gap-3 text-sm">
              <span className="min-w-0 truncate">
                {ACTION_LABEL[a.name] ?? a.name}
                {a.error && <span className="text-destructive-foreground"> — {a.error.code}</span>}
                {a.result && !a.error && Object.keys(a.result).length > 0 && (
                  <span className="font-mono text-xs text-muted-foreground"> {Object.entries(a.result).slice(0, 2).map(([k, val]) => `${k}: ${fmtVal(val)}`).join(" · ")}</span>
                )}
              </span>
              <Badge variant={a.mode === "execute" ? "success" : a.mode === "preview" ? "warning" : "secondary"} size="sm">{MODE_LABEL[a.mode]}</Badge>
            </li>
          ))}
        </ul>
      )}
    </Section>
  );
}
