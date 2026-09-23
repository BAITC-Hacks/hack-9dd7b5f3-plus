/**
 * Replays data/dialogs_sample.json (10 annotated multi-turn dialogs) through the browser MOCK
 * engine — the same code /call runs — and compares every client turn with its annotation:
 * primary scenario, full scenario set, and the reply language of the annotated bot turn.
 * Usage (from frontend/):  npx tsx scripts/replay-dialogs.ts
 * Prints a Markdown table for README §9.2. The mock is lexical (no LLM) — a floor, not the product's number.
 */
import { readFileSync } from "node:fs";
import type { Trace } from "../src/lib/contract";
import { newDialogState, runMockTurn } from "../src/lib/mock/engine";

interface Turn {
  role: "client" | "bot";
  text: string;
  lang?: string;
  scenarios?: string[];
}
interface Dialog {
  dialog_id: string;
  title: string;
  tags: string[];
  turns: Turn[];
}

async function main() {
  const { dialogs } = JSON.parse(readFileSync(new URL("../../data/dialogs_sample.json", import.meta.url), "utf8")) as { dialogs: Dialog[] };
  const total = { turns: 0, primary: 0, full: 0, lang: 0, langN: 0 };
  const rows: string[] = [];
  const misses: string[] = [];

  for (const d of dialogs) {
    const state = newDialogState(`s_${d.dialog_id}`);
    const c = { turns: 0, primary: 0, full: 0, lang: 0, langN: 0 };
    for (let i = 0; i < d.turns.length; i++) {
      const t = d.turns[i];
      if (t.role !== "client") continue;
      let trace: Trace | null = null;
      for await (const ev of runMockTurn(state, { text: t.text, t0: Date.now(), speed: 0 })) if (ev.type === "turn.done") trace = ev.trace;
      const got = trace ? trace.scenarios.map((s) => s.scenario_id) : [];
      const exp = t.scenarios ?? [];
      const full = got.length === exp.length && got.every((x) => exp.includes(x));
      c.turns++;
      if (got[0] === exp[0]) c.primary++;
      if (full) c.full++;
      else misses.push(`${d.dialog_id} turn ${trace?.turn ?? "?"}: expected ${exp.join("+")} got ${got.join("+") || "—"} | ${t.text}`);
      const bot = d.turns[i + 1];
      if (bot?.role === "bot" && bot.lang) {
        c.langN++;
        if (bot.lang === trace?.response_lang) c.lang++;
      }
    }
    total.turns += c.turns;
    total.primary += c.primary;
    total.full += c.full;
    total.lang += c.lang;
    total.langN += c.langN;
    rows.push(`| ${d.dialog_id} | ${d.title} | ${d.tags.join(", ")} | ${c.primary}/${c.turns} | ${c.full}/${c.turns} | ${c.lang}/${c.langN} |`);
  }

  console.log(
    [
      "| Dialog | Title | Tags | Primary | Full set | Reply language |",
      "|---|---|---|---|---|---|",
      ...rows,
      `| **Total** | | | **${total.primary}/${total.turns}** | **${total.full}/${total.turns}** | **${total.lang}/${total.langN}** |`,
    ].join("\n"),
  );
  if (misses.length) console.log(`\nMisses (${misses.length}):\n` + misses.map((m) => `  ${m}`).join("\n"));
}

void main();
