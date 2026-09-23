/** Adapter for the Python router. API credentials stay on the Python server. */
import type { DialogState, Lang } from "./contract";
import { extractSlots, NO, YES, splitParts, type MockRouteResult } from "./mock/router";

type Entry = { id: string; pct: number; name: string };
export interface CoreResult {
  primary: string | null;
  status: "route" | "clarify" | "handoff" | "out_of_scope" | "goodbye";
  intents: Entry[];
  alternatives: Entry[];
  scenarios: Entry[];
  predicted: string[];
  language: Lang;
  model: string;
  route_ms: number;
}
export interface CoreContext {
  history: Array<{ text: string; scenario: string }>;
  active?: string | null;
  last_bot?: string;
}

export async function requestCoreRoute(text: string, context?: CoreContext): Promise<CoreResult> {
  const response = await fetch("/api/core-route", {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text, ...context }), signal: AbortSignal.timeout(30_000),
  });
  const result = await response.json();
  if (!response.ok) throw new Error(result.error ?? `core-llm: HTTP ${response.status}`);
  return result as CoreResult;
}

export function adaptCoreRoute(result: CoreResult, text: string, state: DialogState): MockRouteResult {
  const map = (e: Entry) => ({ scenario_id: e.id, confidence: e.pct / 100 });
  const reason = `LLM ${result.model}: ${result.scenarios.map((e) => `${e.id} (${e.name}) ${e.pct}%`).join(", ") || "нет уверенного сценария"}. Политика ядра: ${result.status}.`;
  // Confirmations execute only when the model kept the pending scenario and
  // did not identify another request. All other utterances use the LLM decision.
  const confirmation = result.status === "route" && state.awaiting?.kind === "confirmation" && result.primary === state.active_scenario && result.intents.length === 1 && text.split(/\s+/).length <= 6;
  const kind = result.status === "goodbye" ? "goodbye" : confirmation && NO.test(text) ? "no" : confirmation && YES.test(text) ? "yes" : "route";
  return {
    kind, parts: splitParts(text), urgent: result.intents.some((e) => ["SC11", "SC15", "SC38"].includes(e.id)),
    snapshots: [result.scenarios.map(map)],
    decision: {
      scenarios: result.intents.map(map), alternatives: result.alternatives.map(map),
      language: result.language, slots: extractSlots(text), is_continuation: kind === "yes" || kind === "no",
      reason, model: result.model, tier: "full", route_status: result.status,
    },
  };
}
