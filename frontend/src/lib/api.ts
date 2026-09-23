/**
 * API layer. `NEXT_PUBLIC_API_MODE=mock` (default) runs everything in the browser;
 * `real` talks to the Go backend (NEXT_PUBLIC_API_URL) using the SSE contract.
 */
import type { DialogState, SupervisorStats, Trace, TurnEvent, TurnRequest } from "./contract";
import { newDialogState, runMockTurn } from "./mock/engine";

export type ApiMode = "mock" | "real";

export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
export const DEFAULT_MODE: ApiMode = (process.env.NEXT_PUBLIC_API_MODE as ApiMode) === "real" ? "real" : "mock";

/* ------------------------------ mock session store ------------------------------ */

const mockSessions = new Map<string, DialogState>();
const mockTraces: Trace[] = [];

function uid(): string {
  return "s_" + Math.random().toString(36).slice(2, 10);
}

export async function createSession(mode: ApiMode): Promise<{ session_id: string; state: DialogState }> {
  if (mode === "real") {
    const r = await fetch(`${API_URL}/api/session`, { method: "POST" });
    if (!r.ok) throw new Error(`session: HTTP ${r.status}`);
    return r.json();
  }
  const id = uid();
  const state = newDialogState(id);
  mockSessions.set(id, state);
  return { session_id: id, state };
}

export function getMockState(session_id: string): DialogState | undefined {
  return mockSessions.get(session_id);
}

/** Run one turn and stream its events. */
export async function* runTurn(mode: ApiMode, req: TurnRequest): AsyncGenerator<TurnEvent> {
  if (mode === "mock") {
    const state = mockSessions.get(req.session_id) ?? newDialogState(req.session_id);
    mockSessions.set(req.session_id, state);
    if (!req.text) {
      yield { type: "error", message: "mock mode needs text (use browser speech recognition)" };
      return;
    }
    for await (const ev of runMockTurn(state, { text: req.text, t0: req.client_t0 ?? Date.now() })) {
      if (ev.type === "turn.done") mockTraces.push(ev.trace);
      yield ev;
    }
    return;
  }
  const r = await fetch(`${API_URL}/api/turn`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
    body: JSON.stringify(req),
  });
  if (!r.ok || !r.body) {
    yield { type: "error", message: `turn: HTTP ${r.status}` };
    return;
  }
  const reader = r.body.getReader();
  const dec = new TextDecoder();
  let buf = "";
  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    buf += dec.decode(value, { stream: true });
    let idx: number;
    while ((idx = buf.indexOf("\n\n")) >= 0) {
      const frame = buf.slice(0, idx);
      buf = buf.slice(idx + 2);
      for (const line of frame.split("\n")) {
        if (!line.startsWith("data:")) continue;
        const json = line.slice(5).trim();
        if (!json) continue;
        try {
          yield JSON.parse(json) as TurnEvent;
        } catch {
          yield { type: "error", message: "bad event: " + json.slice(0, 80) };
        }
      }
    }
  }
}

export async function getSupervisorStats(mode: ApiMode): Promise<SupervisorStats> {
  if (mode === "real") {
    const r = await fetch(`${API_URL}/api/supervisor/stats`);
    if (!r.ok) throw new Error(`stats: HTTP ${r.status}`);
    return r.json();
  }
  return computeStats(mockTraces, mockSessions.size);
}

export function computeStats(traces: Trace[], sessions: number): SupervisorStats {
  const conf = traces.map((t) => t.scenarios[0]?.confidence ?? 0);
  const totals = traces.map((t) => t.latency_ms.total ?? 0).sort((a, b) => a - b);
  const q = (p: number) => (totals.length ? totals[Math.min(totals.length - 1, Math.floor(p * totals.length))] : 0);
  const byScenario = new Map<string, { count: number; sum: number }>();
  const byLang = new Map<string, number>();
  for (const t of traces) {
    const id = t.scenarios[0]?.scenario_id ?? "—";
    const e = byScenario.get(id) ?? { count: 0, sum: 0 };
    e.count++; e.sum += t.scenarios[0]?.confidence ?? 0;
    byScenario.set(id, e);
    byLang.set(t.language, (byLang.get(t.language) ?? 0) + 1);
  }
  return {
    sessions,
    turns: traces.length,
    avg_confidence: conf.length ? conf.reduce((a, b) => a + b, 0) / conf.length : 0,
    low_confidence_turns: conf.filter((c) => c < 0.75).length,
    clarifications: traces.filter((t) => t.policy.action === "clarify").length,
    handoffs: traces.filter((t) => t.policy.action === "handoff" || t.actions.some((a) => a.startsWith("transfer_to_operator"))).length,
    out_of_scope: traces.filter((t) => t.policy.action === "out_of_scope").length,
    median_total_ms: q(0.5),
    p95_total_ms: q(0.95),
    by_scenario: [...byScenario.entries()].map(([scenario_id, e]) => ({ scenario_id, count: e.count, avg_confidence: e.sum / e.count })).sort((a, b) => b.count - a.count),
    by_language: [...byLang.entries()].map(([language, count]) => ({ language: language as Trace["language"], count })),
    recent: traces.slice(-20).reverse(),
  };
}
