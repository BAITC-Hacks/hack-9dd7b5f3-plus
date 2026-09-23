/**
 * Bagyt · Voice Router — frontend ⇄ backend contract (Mock-Driven Development).
 *
 * The frontend is written against THIS file. The Go backend must produce the same
 * shapes. Transport for a turn: `POST /api/turn` → `text/event-stream`, one JSON
 * TurnEvent per `data:` line, in the order listed in `TurnEvent` below.
 *
 * Trace format at the end of every turn follows data/README.md → "Trace".
 */

export type Lang = "ru" | "kk" | "mixed";
export type ReplyLang = "ru" | "kk";

export type ScenarioId = string; // "SC01".."SC40" | "SYS_OUT_OF_SCOPE" | "SYS_UNCLEAR" | "SYS_GOODBYE"

export type Priority = "normal" | "high" | "urgent";

/** One ranked scenario from the router. */
export interface RoutedScenario {
  scenario_id: ScenarioId;
  confidence: number; // 0..1
  reason?: string;
}

export interface Candidate {
  scenario_id: ScenarioId;
  confidence: number; // 0..1, live/partial value
}

/** Router output (README "Уровень 2 — LLM-маршрутизатор"). */
export interface RouterDecision {
  scenarios: RoutedScenario[]; // ordered: urgent first, then mention order
  alternatives: Candidate[];
  language: Lang;
  slots: Record<string, unknown>;
  is_continuation: boolean;
  reason: string;
  model?: string; // e.g. "mock" | "gpt-4.1-mini"
  tier?: "fast" | "full";
  /** Authoritative policy from core-llm; absent for the lexical mock. */
  route_status?: "route" | "clarify" | "handoff" | "out_of_scope" | "goodbye";
}

/** Decision policy verdict (README "Политика принятия решений"). */
export type PolicyAction = "run" | "continue" | "clarify" | "handoff" | "out_of_scope" | "goodbye";

export interface PolicyVerdict {
  action: PolicyAction;
  scenario_id: ScenarioId | null; // the scenario being executed now
  queue?: string; // handoff queue
  reason: string;
  stack: ScenarioId[]; // interrupted scenarios waiting to be resumed
  low_conf_streak: number;
}

export type ActionMode = "read" | "preview" | "execute";

export interface ActionCall {
  name: string;
  mode: ActionMode;
  input?: Record<string, unknown>;
  result?: Record<string, unknown>;
  error?: { code: string; message: string };
  ms?: number;
}

/** Pipeline stages, in order. Backend reports `ms` for each. */
export type Stage = "stt" | "triage" | "router" | "policy" | "executor" | "response" | "tts_first_audio" | "total";

export type LatencyMs = Partial<Record<Stage, number>>;

/** Dialog state kept per session (README "Состояние диалога"). */
export interface DialogState {
  session_id: string;
  language: ReplyLang; // language we answer in
  client_id: string | null;
  client_name: string | null;
  active_scenario: ScenarioId | null;
  stack: ScenarioId[];
  slots: Record<string, unknown>;
  awaiting: { kind: "slot"; slot: string } | { kind: "confirmation"; action: string } | null;
  low_conf_streak: number;
  turn: number;
  ended: boolean;
}

/** Trace shown after each client turn — exactly the README format + extras. */
export interface Trace {
  turn: number;
  transcript: string;
  language: Lang;
  scenarios: RoutedScenario[];
  alternatives: Candidate[];
  reason: string;
  slots: Record<string, unknown>;
  actions: string[]; // "find_client", "book_appointment:preview"
  latency_ms: LatencyMs;
  // extras (not in README, useful for the supervisor)
  policy: PolicyVerdict;
  response_text: string;
  response_lang: ReplyLang;
  model?: string;
  tier?: "fast" | "full";
  candidates_history?: Candidate[][]; // live confidence snapshots, in order
}

/* ------------------------------------------------------------------ */
/* Event stream for one turn                                            */
/* ------------------------------------------------------------------ */

export type TurnEvent =
  | { type: "turn.start"; turn: number; session_id: string; t0: number }
  | { type: "stt.partial"; text: string }
  | { type: "stt.final"; text: string; language: Lang; ms: number }
  | { type: "triage"; language: Lang; urgent: boolean; parts: string[]; normalized: string; ms: number }
  /** May be emitted several times before `router.decision` (speculative / streaming). */
  | { type: "router.candidates"; candidates: Candidate[]; partial: boolean }
  | { type: "router.decision"; decision: RouterDecision; ms: number }
  | { type: "policy"; verdict: PolicyVerdict; ms: number }
  | { type: "action"; call: ActionCall }
  | { type: "state"; state: DialogState }
  | { type: "response.delta"; text: string }
  | { type: "response.final"; text: string; language: ReplyLang; ms: number }
  /** Real backend: base64 audio (audio/mpeg or audio/wav). Mock: `browser_tts: true` → speak via speechSynthesis. */
  | { type: "tts.audio"; audio_base64?: string; mime?: string; browser_tts?: boolean; ms_first_audio: number }
  | { type: "turn.done"; trace: Trace; latency_ms: LatencyMs }
  | { type: "error"; message: string };

/* ------------------------------------------------------------------ */
/* REST shapes                                                          */
/* ------------------------------------------------------------------ */

export interface TurnRequest {
  session_id: string;
  text?: string; // text fallback or browser-STT result
  audio_base64?: string; // webm/opus from MediaRecorder (real backend does STT)
  audio_mime?: string;
  lang_hint?: ReplyLang;
  client_t0?: number; // epoch ms when the client finished speaking (for end-to-end latency)
}

export interface SessionCreateResponse {
  session_id: string;
  state: DialogState;
}

export interface SupervisorStats {
  sessions: number;
  turns: number;
  avg_confidence: number;
  low_confidence_turns: number; // confidence < 0.75
  clarifications: number; // SYS_UNCLEAR
  handoffs: number;
  out_of_scope: number;
  median_total_ms: number;
  p95_total_ms: number;
  by_scenario: Array<{ scenario_id: ScenarioId; count: number; avg_confidence: number }>;
  by_language: Array<{ language: Lang; count: number }>;
  recent: Trace[];
}

export const CONFIDENCE_RUN = 0.75;
export const CONFIDENCE_CLARIFY = 0.45;
export const AS_OF_DATE = "2026-10-01";
