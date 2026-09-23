/**
 * Typed access to the official starter kit (copied from ../data into src/data).
 * Source of truth is /data at the repo root — re-copy if it changes.
 */
import scenariosJson from "@/data/scenarios.json";
import slotsJson from "@/data/slots.json";
import kbJson from "@/data/knowledge_base.json";
import backendJson from "@/data/mock_backend.json";
import namesJson from "@/data/scenario_names.json";
import type { Priority, ReplyLang, ScenarioId } from "./contract";

export interface NotThisIf {
  condition: string;
  use_instead: string;
}

export interface Scenario {
  scenario_id: string;
  slug: string;
  name: string;
  domain: string;
  category: string;
  description: string;
  not_this_if: NotThisIf[];
  priority: Priority;
  fast_path_eligible: boolean;
  requires_identification: boolean;
  slots: { required: string[]; optional: string[] };
  actions: string[];
  requires_confirmation: boolean;
  handoff: { when: string; queue: string } | null;
  examples: { ru: string[]; kk: string[] };
  responses: { ru: { opening: string; closing: string }; kk: { opening: string; closing: string } };
}

export interface SystemIntent {
  id: string;
  description: string;
  behavior: string;
  response: { ru: string; kk: string };
}

export interface Slot {
  name: string;
  type: string;
  description: string;
  pattern?: string;
  values?: Array<string | number>;
  prompt: { ru: string; kk: string };
}

export interface Client {
  client_id: string;
  full_name: string;
  phone: string;
  iin: string;
  city: string;
  email: string;
  address: string;
  bm_class: string;
  preferred_language: string;
}

export interface Policy {
  policy_number: string;
  client_id: string;
  product: string;
  start_date: string;
  end_date: string;
  premium: number | null;
  details: Record<string, unknown>;
}

export interface Claim {
  claim_number: string;
  client_id: string;
  policy_number: string;
  claim_type: string;
  incident_date: string;
  status: string;
  approved_amount?: number;
  next_step: string;
  missing_documents?: string[];
  decision_due?: string;
}

export interface Payment {
  payment_id: string;
  client_id: string;
  date: string;
  amount: number;
  product: string;
  status: string;
  policy_number: string | null;
  note?: string;
}

/* eslint-disable @typescript-eslint/no-explicit-any */
export const scenarios: Scenario[] = (scenariosJson as any).scenarios;
export const systemIntents: SystemIntent[] = [...(scenariosJson as any).system_intents, {
  id: "SYS_GREETING",
  description: "Greeting without a substantive request",
  behavior: "Greet in the client's language and preserve dialog context",
  response: { ru: "Здравствуйте, чем могу помочь?", kk: "Сәлеметсіз бе, қалай көмектесе аламын?" },
}];
export const slots: Slot[] = (slotsJson as any).slots;
export const kb: any = kbJson;
export const backend: { clients: Client[]; policies: Policy[]; claims: Claim[]; payments: Payment[] } = backendJson as any;
export const scenarioNames: { ru: Record<string, string>; kk: Record<string, string> } = namesJson as any;
/* eslint-enable @typescript-eslint/no-explicit-any */

const byId = new Map(scenarios.map((s) => [s.scenario_id, s]));
const slotByName = new Map(slots.map((s) => [s.name, s]));

export function scenarioById(id: ScenarioId): Scenario | undefined {
  return byId.get(id);
}

export function slotByNameOf(name: string): Slot | undefined {
  return slotByName.get(name);
}

export function systemIntentById(id: string): SystemIntent | undefined {
  return systemIntents.find((s) => s.id === id);
}

export function isSystemIntent(id: ScenarioId): boolean {
  return id.startsWith("SYS_");
}

/** Human name of a scenario in the reply language (falls back to the English name). */
export function scenarioLabel(id: ScenarioId, lang: ReplyLang = "ru"): string {
  if (isSystemIntent(id)) {
    const map: Record<string, { ru: string; kk: string }> = {
      SYS_OUT_OF_SCOPE: { ru: "Вне компетенции", kk: "Құзырет шегінен тыс" },
      SYS_UNCLEAR: { ru: "Уточнение", kk: "Нақтылау" },
      SYS_GREETING: { ru: "Приветствие", kk: "Сәлемдесу" },
      SYS_GOODBYE: { ru: "Завершение", kk: "Аяқтау" },
    };
    return map[id]?.[lang] ?? id;
  }
  return scenarioNames[lang]?.[id] ?? byId.get(id)?.name ?? id;
}

export const PRIORITY_RANK: Record<Priority, number> = { urgent: 0, high: 1, normal: 2 };

export const URGENT_IDS = scenarios.filter((s) => s.priority === "urgent").map((s) => s.scenario_id);
