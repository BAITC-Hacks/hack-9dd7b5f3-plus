/**
 * Runs the browser MOCK router over data/dev_utterances.json and writes predictions.json
 * for the official evaluate.py. Usage (from frontend/):  npx tsx scripts/eval-mock.ts
 * The mock is lexical (no LLM) — this number is a floor, not the product's accuracy.
 */
import { readFileSync, writeFileSync } from "node:fs";
import { mockRoute } from "../src/lib/mock/router";

const dev = JSON.parse(readFileSync(new URL("../../data/dev_utterances.json", import.meta.url), "utf8")) as {
  utterances: Array<{ id: string; text: string; expected: string[]; lang: string; type: string }>;
};
const preds: Record<string, string[]> = {};
let t = 0;
for (const u of dev.utterances) {
  const t0 = performance.now();
  const r = mockRoute(u.text, { awaiting: false });
  t += performance.now() - t0;
  preds[u.id] = r.decision.scenarios.map((s) => s.scenario_id);
}
writeFileSync(new URL("../../data/predictions_mock.json", import.meta.url), JSON.stringify(preds, null, 1));
console.log(`routed ${dev.utterances.length} utterances, mean ${(t / dev.utterances.length).toFixed(2)} ms each → data/predictions_mock.json`);
