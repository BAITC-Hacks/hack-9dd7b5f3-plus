"use client";
/** Supervisor console — shared helpers. Minimal: sentence-case labels, numbers in mono. */
import type React from "react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { ActionCall, ActionMode, LatencyMs, PolicyAction, PolicyVerdict, RouterDecision, Stage, Trace } from "@/lib/contract";
import type { LiveTurn } from "@/lib/store";

/** The turn shown on the right: the in-progress one, else the last finished trace. */
export interface TurnView {
  source: "live" | "trace" | "none";
  turn: number;
  stage: Stage | "done" | null;
  transcript: string;
  language?: string;
  parts?: string[];
  urgent?: boolean;
  candidates: { scenario_id: string; confidence: number }[];
  decision?: RouterDecision;
  verdict?: PolicyVerdict;
  actions: ActionCall[];
  latency: LatencyMs;
  error?: string;
}

export function toTurnView(live: LiveTurn | null, lastTrace: Trace | undefined): TurnView {
  if (live) {
    return { source: "live", turn: live.turn, stage: live.stage, transcript: live.transcript, language: live.language, parts: live.parts, urgent: live.urgent, candidates: live.candidates, decision: live.decision, verdict: live.verdict, actions: live.actions, latency: live.latency, error: live.error };
  }
  if (lastTrace) {
    const t = lastTrace;
    return {
      source: "trace", turn: t.turn, stage: "done", transcript: t.transcript, language: t.language,
      candidates: t.scenarios.map((s) => ({ scenario_id: s.scenario_id, confidence: s.confidence })).concat(t.alternatives),
      decision: { scenarios: t.scenarios, alternatives: t.alternatives, language: t.language, slots: t.slots, is_continuation: t.policy.action === "continue", reason: t.reason, model: t.model, tier: t.tier },
      verdict: t.policy,
      actions: t.actions.map((a) => { const [name, mode] = a.split(":"); return { name, mode: (mode as ActionMode) || "read" }; }),
      latency: t.latency_ms,
    };
  }
  return { source: "none", turn: 0, stage: null, transcript: "", candidates: [], actions: [], latency: {} };
}

export const TARGET_MS = 1500;

export function fmtMs(v: number | undefined | null): string {
  return typeof v === "number" && Number.isFinite(v) ? Math.round(v).toLocaleString("ru-RU") : "—";
}
export function fmtPct(v: number | undefined | null): string {
  return typeof v === "number" && Number.isFinite(v) ? `${Math.round(v * 100)}%` : "—";
}

export const POLICY_LABEL: Record<PolicyAction, string> = {
  run: "Запустил сценарий",
  continue: "Продолжил сценарий",
  clarify: "Переспросил",
  handoff: "Передал оператору",
  out_of_scope: "Не наша тема",
  goodbye: "Попрощался",
  greeting: "Поздоровался",
};
const POLICY_VARIANT: Record<PolicyAction, "success" | "info" | "warning" | "error" | "secondary"> = {
  run: "success", continue: "info", clarify: "warning", handoff: "error", out_of_scope: "secondary", goodbye: "secondary", greeting: "info",
};

export function PolicyBadge({ action, size = "default" }: { action?: PolicyAction | null; size?: "sm" | "default" | "lg" }) {
  if (!action) return <span className="text-muted-foreground">—</span>;
  return <Badge variant={POLICY_VARIANT[action]} size={size}>{POLICY_LABEL[action]}</Badge>;
}

/** Plain card with a small title. */
export function Section({ title, hint, right, children, className, bodyClassName }: { title: string; hint?: string; right?: React.ReactNode; children: React.ReactNode; className?: string; bodyClassName?: string }) {
  return (
    <section className={cn("rounded-xl border border-border bg-card", className)}>
      <div className="flex items-center justify-between gap-3 px-4 pt-3.5 pb-2">
        <div className="min-w-0">
          <div className="text-sm font-medium">{title}</div>
          {hint && <div className="truncate text-xs text-muted-foreground">{hint}</div>}
        </div>
        {right && <div className="flex shrink-0 items-center gap-2">{right}</div>}
      </div>
      <div className={cn("px-4 pb-4", bodyClassName)}>{children}</div>
    </section>
  );
}

/** Confidence bar, 6px. */
export function Bar({ value, fill = "bg-s6", className }: { value: number; fill?: string; className?: string }) {
  const w = Math.max(0, Math.min(100, Math.round(value * 100)));
  return (
    <div className={cn("h-1.5 w-full overflow-hidden rounded-full bg-white/[.06]", className)}>
      <div className={cn("h-full rounded-full transition-[width] duration-200 ease-out", fill)} style={{ width: `${w}%` }} />
    </div>
  );
}

export function Row({ k, children }: { k: string; children: React.ReactNode }) {
  return (
    <div className="flex items-baseline justify-between gap-4 border-b border-border py-2 text-sm last:border-b-0">
      <span className="shrink-0 text-muted-foreground">{k}</span>
      <span className="min-w-0 truncate text-right">{children}</span>
    </div>
  );
}

export function Dash() {
  return <span className="text-muted-foreground">—</span>;
}

export function Metric({ label, value, unit, hint, warn }: { label: string; value: string; unit?: string; hint?: string; warn?: boolean }) {
  return (
    <div className="rounded-xl border border-border bg-card px-4 py-3.5">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className={cn("mt-1 text-2xl font-medium tabular-nums", warn && "text-warning-foreground")}>
        {value}{unit && <span className="ml-1 text-sm font-normal text-muted-foreground">{unit}</span>}
      </div>
      {hint && <div className="mt-0.5 text-xs text-muted-foreground">{hint}</div>}
    </div>
  );
}

/** Trace in README format. */
export function readmeTrace(t: Trace) {
  return { turn: t.turn, transcript: t.transcript, language: t.language, scenarios: t.scenarios, alternatives: t.alternatives, reason: t.reason, slots: t.slots, actions: t.actions, latency_ms: t.latency_ms };
}

/** Human names for mock actions and slots. */
export const ACTION_LABEL: Record<string, string> = {
  find_client: "Поиск клиента", get_policies: "Список полисов", get_policy: "Данные полиса", get_bm_class: "Класс бонус-малус",
  calc_ogpo_price: "Расчёт ОГПО", calc_casco_price: "Расчёт КАСКО", calc_travel_price: "Расчёт страховки для поездки", calc_property_price: "Расчёт страховки жилья", calc_accident_price: "Расчёт страховки от НС",
  create_policy: "Оформление полиса", renew_policy: "Продление полиса", update_policy: "Изменение полиса", cancel_policy: "Расторжение полиса",
  create_claim: "Регистрация страхового случая", get_claim: "Статус страхового случая", create_dispute: "Регистрация несогласия",
  book_inspection: "Запись на осмотр", book_appointment: "Запись к врачу", check_coverage: "Проверка покрытия", list_clinics: "Клиники",
  resend_documents: "Повторная отправка полиса", check_payment: "Проверка платежа", update_contact: "Обновление контактов", request_document: "Заказ справки",
  get_offices: "Офисы", kb_lookup: "База знаний", send_sms: "SMS", create_callback: "Обратный звонок", create_complaint: "Жалоба", report_fraud: "Сообщение о мошенничестве", transfer_to_operator: "Перевод на оператора",
};
export const MODE_LABEL: Record<ActionMode, string> = { read: "чтение", preview: "предпросмотр", execute: "выполнено" };
export const SLOT_LABEL: Record<string, string> = {
  phone: "номер телефона", iin: "ИИН", policy_number: "номер полиса", claim_number: "номер заявления", vehicle_plate: "госномер", culprit_vehicle_plate: "госномер виновника",
  vehicle_type: "тип авто", region: "регион", drivers_iin: "ИИН водителей", new_driver_iin: "ИИН нового водителя", car_value: "стоимость авто", car_year: "год выпуска", franchise: "франшиза",
  product_type: "вид страховки", trip_country: "страна", trip_start: "начало поездки", trip_end: "конец поездки", travelers_count: "число путешественников", traveler_max_age: "возраст старшего",
  property_type: "тип жилья", property_address: "адрес", sum_insured: "страховая сумма", incident_date: "дата события", incident_description: "что случилось", injured: "пострадавшие", location: "местоположение",
  city: "город", email: "почта", contact_field: "что меняем", new_value: "новое значение", payment_date: "дата платежа", payment_amount: "сумма платежа", callback_time: "время звонка",
  doctor_specialty: "врач", service_name: "услуга", preferred_date: "дата", company_name: "компания", employees_count: "число сотрудников", cancel_reason: "причина", complaint_text: "суть жалобы", fraud_details: "детали", document_type: "документ", topic: "тема",
};
