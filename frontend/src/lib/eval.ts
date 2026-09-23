/**
 * Port of data/evaluate.py — the official metrics:
 *   primary_accuracy  first predicted scenario == first expected
 *   full_match        set(predicted) == set(expected)
 *   intent_recall     share of expected scenarios found (multi-intent only)
 * grouped by "all", "lang=…", "type=…".
 */
import devJson from "@/data/dev_utterances.json";
import { API_URL, type ApiMode } from "./api";
import { mockRoute } from "./mock/router";

export interface DevUtterance {
  id: string;
  text: string;
  lang: string;
  expected: string[];
  type: string;
}

export const devUtterances: DevUtterance[] = (devJson as { utterances: DevUtterance[] }).utterances;

export interface EvalGroup {
  key: string;
  n: number;
  primary: number; // 0..1
  full: number; // 0..1
}

export interface EvalResult {
  groups: EvalGroup[];
  intent_recall: number | null;
  errors: Array<{ id: string; text: string; expected: string[]; got: string[]; lang: string; type: string }>;
  mean_ms: number;
  model: string;
}

export function scorePredictions(preds: Record<string, string[]>, utts: DevUtterance[] = devUtterances): Omit<EvalResult, "mean_ms" | "model"> {
  const groups = new Map<string, { n: number; primary: number; full: number }>();
  let recallHit = 0;
  let recallTotal = 0;
  const errors: EvalResult["errors"] = [];
  for (const u of utts) {
    const got = preds[u.id] ?? [];
    const primary = got.length > 0 && got[0] === u.expected[0];
    const full = sameSet(got, u.expected);
    if (u.type === "multi_intent") {
      recallHit += u.expected.filter((e) => got.includes(e)).length;
      recallTotal += u.expected.length;
    }
    for (const key of ["all", `lang=${u.lang}`, `type=${u.type}`]) {
      const g = groups.get(key) ?? { n: 0, primary: 0, full: 0 };
      g.n++;
      if (primary) g.primary++;
      if (full) g.full++;
      groups.set(key, g);
    }
    if (!full) errors.push({ id: u.id, text: u.text, expected: u.expected, got, lang: u.lang, type: u.type });
  }
  const order = ["all", ...[...groups.keys()].filter((k) => k.startsWith("lang=")).sort(), ...[...groups.keys()].filter((k) => k.startsWith("type=")).sort()];
  return {
    groups: order.map((key) => { const g = groups.get(key)!; return { key, n: g.n, primary: g.primary / g.n, full: g.full / g.n }; }),
    intent_recall: recallTotal ? recallHit / recallTotal : null,
    errors,
  };
}

function sameSet(a: string[], b: string[]): boolean {
  const sa = new Set(a);
  const sb = new Set(b);
  if (sa.size !== sb.size) return false;
  for (const x of sa) if (!sb.has(x)) return false;
  return true;
}

/** Run the dev set through the router (mock: in the browser; real: POST /api/eval/run). */
export async function runEval(mode: ApiMode): Promise<EvalResult> {
  if (mode === "real") {
    const r = await fetch(`${API_URL}/api/eval/run`, { method: "POST" });
    if (!r.ok) throw new Error(`eval: HTTP ${r.status}`);
    return r.json();
  }
  const preds: Record<string, string[]> = {};
  const t0 = performance.now();
  for (const u of devUtterances) preds[u.id] = mockRoute(u.text, { awaiting: false }).decision.scenarios.map((s) => s.scenario_id);
  const mean = (performance.now() - t0) / devUtterances.length;
  return { ...scorePredictions(preds), mean_ms: mean, model: "mock-lexical" };
}
