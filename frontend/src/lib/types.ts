// Shared types mirroring the Go backend contract (docs/SPEC.md, backend/internal/*).
// Go nil slices/maps serialize as null, so collections are typed `T[] | null`.

export type Lang = "ru" | "kk" | "mixed" | (string & {});
export type Path = "llm" | "fast" | "mock" | "fallback" | (string & {});
export type PolicyAction = "proceed" | "clarify" | "handoff" | "out_of_scope" | "goodbye" | (string & {});
export type Dict = Record<string, unknown>;

export interface Health {
  ok: boolean;
  status?: string;
  mock_mode: boolean;
  router: string;
  stt: string;
  tts: string;
  uptime_s?: number;
}

export interface AppConfig {
  llm: { provider: string; model: string; base_url?: string; has_key: boolean; json_mode?: boolean };
  stt: { provider: string; model: string; base_url?: string; has_key: boolean; language?: string };
  tts: {
    provider: string;
    model: string;
    voice?: string;
    model_kk?: string;
    voice_kk?: string;
    sample_rate: number;
    has_key: boolean;
  };
  fast_path: string;
  policy: { proceed_min: number; clarify_min: number; fast_path_min_score: number; fast_path_min_margin: number };
  router?: string;
  tts_sample_rate?: number;
  as_of_date?: string;
  debug?: boolean;
}

export interface ScenarioPick {
  id: string;
  confidence: number;
  reason?: string;
}
export interface Alt {
  id: string;
  confidence: number;
}
export interface ActionRequest {
  name: string;
  args?: Dict | null;
  mode?: string;
}
export interface Handoff {
  queue: string;
  summary?: string;
}
export interface Decision {
  language: string;
  scenarios: ScenarioPick[] | null;
  alternatives: Alt[] | null;
  is_continuation: boolean;
  slots: Dict | null;
  actions: ActionRequest[] | null;
  handoff: Handoff | null;
  reply: string;
}
export interface Verdict {
  action: PolicyAction;
  confidence: number;
  notes?: string[] | null;
}
export interface FastPathCheck {
  eligible: boolean;
  reason: string;
  candidate?: string;
  score?: number;
  margin?: number;
}
export interface Signals {
  language: string;
  kk_share: number;
  normalized: string;
  entities: Record<string, string> | null;
  urgent: string[] | null;
  multi_intent_markers: string[] | null;
  confirmation: string;
  goodbye: boolean;
  greeting_only: boolean;
  operator_request: boolean;
  robot_question: boolean;
  out_of_scope_hints: string[] | null;
}
export interface Candidate {
  id: string;
  score: number;
  bm25: number;
  terms: string[] | null;
}
export interface Fact {
  name: string;
  args?: Dict | null;
  result?: Dict | null;
  error?: string;
  note?: string;
}
export type ActionMode = "execute" | "preview" | "handoff" | "cancelled" | (string & {});
export interface ActionRecord {
  name: string;
  args?: Dict | null;
  mode: ActionMode;
  result?: Dict | null;
  error?: string;
  note?: string;
  ms?: number;
}
export interface PendingAction {
  name: string;
  args: Dict | null;
  preview?: Dict | null;
  scenario_id: string;
}
export interface TurnView {
  role: string;
  text: string;
}
export interface SessionState {
  id: string;
  created_at?: string;
  channel?: string;
  turn_index: number;
  language: string;
  client_id?: string;
  client_name?: string;
  active_scenario?: string;
  stack: string[] | null;
  slots: Dict | null;
  pending_confirmation?: PendingAction | null;
  clarify_count: number;
  handoff?: Handoff | null;
  closed: boolean;
  turns?: TurnView[] | null;
}

export type TimingKey =
  | "stt"
  | "triage"
  | "retrieval"
  | "facts"
  | "route"
  | "llm_route"
  | "llm_ttft"
  | "llm_total"
  | "followup_llm"
  | "tts_first_byte"
  | "first_audio"
  | "total";
export type Timings = Partial<Record<TimingKey, number>> & Record<string, number | undefined>;

export interface LLMUsage {
  prompt_tokens?: number;
  completion_tokens?: number;
  total_tokens?: number;
}
export interface LLMMessage {
  role: string;
  content: string;
}
export interface LLMInfo {
  provider: string;
  model: string;
  usage?: LLMUsage | null;
  ttft_ms: number;
  total_ms: number;
  repaired?: boolean;
  raw?: string;
  messages?: LLMMessage[] | null;
  error?: string;
}
export interface Shadow {
  llm_primary: string;
  confidence: number;
  agree: boolean;
  ms: number;
  error?: string;
}

export interface Trace {
  session_id: string;
  turn: number;
  at: string;
  input: { source: string; transcript: string; stt_provider?: string; stt_model?: string; audio_ms?: number };
  language: { detected: string; kk_share: number; reply: string };
  triage: Signals;
  retrieval: Candidate[] | null;
  path: Path;
  fast_path: FastPathCheck;
  decision: Decision | null;
  policy: Verdict;
  facts?: Fact[] | null;
  actions: ActionRecord[] | null;
  state: SessionState;
  reply: { text: string; lang: string; sentences: number; follow_up: boolean };
  timings: Timings;
  llm?: LLMInfo | null;
  shadow?: Shadow | null;
  errors?: string[] | null;
}

/** A trace as it is being assembled from live events (everything optional). */
export type LiveTrace = Partial<Omit<Trace, "timings">> & { timings: Timings };

/** What the trace panel renders — a finished record or a turn in flight. */
export interface TraceView {
  turn: number;
  transcript: string;
  source?: string;
  reply: string;
  replyDone: boolean;
  trace: LiveTrace;
  done: boolean;
  provisionalPath?: Path;
  error?: string;
  llmStream?: string;
  speak?: string[];
  client?: { t0?: number; firstAudio?: number; e2e?: number };
  at?: number;
}

export interface TurnRecord {
  session_id: string;
  turn: number;
  at: string;
  transcript: string;
  language: string;
  scenarios: string[] | null;
  confidence: number;
  path: Path;
  policy_action: PolicyAction;
  handoff: boolean;
  fast_path_agree?: boolean | null;
  timings: Timings | null;
  trace?: Trace | null;
}
export interface SessionSummary {
  id: string;
  created_at: string;
  channel: string;
  turns: number;
  last_at: string;
  title: string;
  language: string;
  client_id?: string;
  handoff: boolean;
}
export interface SessionDetail {
  session: SessionSummary;
  turns: TurnRecord[] | null;
  state?: SessionState;
}
export interface TurnResponse {
  session_id: string;
  turn: number;
  reply: string;
  language: string;
  scenarios: string[] | null;
  confidence: number;
  path: Path;
  timings: Timings;
  trace: Trace;
  error?: string;
}
export interface RouteResponse {
  scenarios: string[] | null;
  confidence: number;
  path: Path;
  route_ms: number;
  decision: Decision | null;
  verdict: Verdict;
  candidates: Candidate[] | null;
  signals: Signals;
  fast_path: FastPathCheck;
  llm?: LLMInfo | null;
  error?: string;
}

export interface EvalGroup {
  n: number;
  primary: number;
  full: number;
  primary_acc: number;
  full_match: number;
}
export interface EvalRow {
  id: string;
  text: string;
  lang: string;
  expected: string[] | null;
  type: string;
  got: string[] | null;
  primary_ok: boolean;
  full_ok: boolean;
  confidence: number;
  path: Path;
  route_ms: number;
  reason?: string;
  error?: string;
}
export interface EvalReport {
  ran_at: string;
  router: string;
  n: number;
  groups: Record<string, EvalGroup>;
  intent_recall: number;
  latency_p50_ms: number;
  latency_p90_ms: number;
  latency_max_ms: number;
  by_path: Record<string, number> | null;
  rows: EvalRow[] | null;
  duration_ms: number;
}
export interface EvalLast {
  running: boolean;
  progress: [number, number];
  report: EvalReport | null;
}

export interface Percentiles {
  n: number;
  p50: number;
  p90: number;
  avg: number;
  max: number;
}
export interface Stats {
  turns: number;
  sessions: number;
  by_scenario: Record<string, number> | null;
  by_language: Record<string, number> | null;
  by_path: Record<string, number> | null;
  by_policy: Record<string, number> | null;
  low_confidence: number;
  handoffs: number;
  fast_path_checked: number;
  fast_path_agree: number;
  timings: Record<string, Percentiles> | null;
  first_audio_by_path: Record<string, Percentiles> | null;
  uncertain: TurnRecord[] | null;
  recent: TurnRecord[] | null;
}

export interface Boundary {
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
  not_this_if: Boundary[] | null;
  priority: string;
  fast_path_eligible: boolean;
  requires_identification: boolean;
  slots: { required: string[] | null; optional: string[] | null };
  actions: string[] | null;
  requires_confirmation: boolean;
  handoff: { when: string; queue: string } | null;
  examples: Record<string, string[] | null> | null;
  responses: Record<string, { opening: string; closing: string }> | null;
}
export interface SystemIntent {
  id: string;
  description: string;
  behavior: string;
  response: Record<string, string> | null;
}
export interface CatalogResponse {
  as_of_date: string;
  scenarios: Scenario[] | null;
  system_intents: SystemIntent[] | null;
  dir?: string;
}

// ---------------------------------------------------------------- events

export type EventType =
  | "session"
  | "turn_start"
  | "triage"
  | "retrieval"
  | "facts"
  | "fast_path"
  | "llm_start"
  | "llm_delta"
  | "llm_error"
  | "route"
  | "reply_delta"
  | "reply_done"
  | "action"
  | "handoff"
  | "speak"
  | "tts_first_byte"
  | "tts_sentence"
  | "tts_error"
  | "audio_end"
  | "stt_partial"
  | "stt_start"
  | "stt_final"
  | "stt_error"
  | "stt_realtime"
  | "turn_done"
  | "shadow"
  | "turn_error"
  | "error"
  | "pong";

export interface VREvent<T = unknown> {
  type: EventType | (string & {});
  session_id?: string;
  turn?: number;
  t: number;
  data?: T;
}

export interface SessionEventData {
  session_id: string;
  config: AppConfig;
  audio_out: { sample_rate: number; format: string };
  state: SessionState;
}
export interface TurnStartData {
  text: string;
  source: string;
}
export interface RetrievalData {
  candidates: Candidate[] | null;
  ms: number;
}
export interface LLMStartData {
  model: string;
  prompt_chars?: number;
  messages?: LLMMessage[] | null;
  follow_up?: boolean;
}
export interface RouteData {
  decision: Decision;
  verdict: Verdict;
  route_ms: number;
  since_t0_ms: number;
  language: string;
}
export interface ReplyDeltaData {
  text: string;
  follow_up?: boolean;
}
export interface ReplyDoneData {
  text: string;
  lang: string;
}
export interface SpeakData {
  text: string;
  lang: string;
  fallback?: boolean;
}
export interface TTSFirstByteData {
  tts_ms: number;
  first_audio_ms: number;
  sentence: string;
}
export interface STTFinalData {
  text: string;
  ms: number;
  provider: string;
  model: string;
  language: string;
  audio_ms: number;
}
export interface STTErrorData {
  error: string;
  stage: string;
  fallback?: string;
}
export interface ShadowData {
  fast_primary: string;
  shadow: Shadow;
}
export interface TurnErrorData {
  error: string;
  empty?: boolean;
}
export interface ErrorData {
  error: string;
  text?: string;
}
