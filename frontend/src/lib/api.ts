export const API = (
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"
).replace(/\/$/, "");
export type Alternative = { scenario_id: string; reason: string };
export type Decision = {
  scenario_id: string;
  status: "route" | "clarify" | "handoff";
  confidence: number;
  language: string;
  reason: string;
  alternatives: Alternative[];
  slots: { name: string; value: string }[];
  pending: string[];
};
export type Call = {
  attempt: number;
  provider: string;
  model: string;
  latency_ms: number;
  prompt_tokens: number;
  completion_tokens: number;
  cached_tokens: number;
  error?: string;
};
export type Timing = {
  routing_ms: number;
  policy_ms: number;
  server_ms: number;
  stt_ms?: number;
  tts_first_byte_ms?: number;
  end_to_audio_ms?: number;
  input_kind: string;
};
export type Turn = {
  pending_topics: string[];
  evidence: { source: string; path: string; value: unknown }[];
  id: string;
  text: string;
  reply: string;
  decision: Decision;
  scenario_name: string;
  previous_scenario: string;
  topic_changed: boolean;
  source: string;
  calls: Call[];
  timing: Timing;
  warnings: string[];
  created_at: string;
};
export type Session = {
  id: string;
  turns: Turn[];
  active: string;
  pending: string[];
  created_at: string;
};
export type Health = {
  status: string;
  provider: string;
  model: string;
  speech_provider: string;
  speech_ready: boolean;
  catalog_source: string;
  catalog_count: number;
  catalog_hash: string;
  storage: string;
  max_turns: number;
};
export type Scenario = {
  id: string;
  name: string;
  description: string;
  boundaries: string;
  examples: string[];
  response_ru: string;
  response_kk: string;
  requires_confirmation: boolean;
};
export type TraceEvent = {
  type: string;
  elapsed_ms: number;
  data: Record<string, unknown>;
};
export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API}${path}`, {
    ...init,
    headers: {
      ...(init?.body instanceof FormData
        ? {}
        : { "Content-Type": "application/json" }),
      ...init?.headers,
    },
    cache: "no-store",
  });
  if (!response.ok) {
    const err = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(err.error || `HTTP ${response.status}`);
  }
  return response.json();
}
export async function streamTurn(
  body: {
    session_id: string;
    text: string;
    input_kind: string;
    stt_ms?: number;
  },
  onEvent: (event: TraceEvent) => void,
  signal: AbortSignal,
): Promise<{ session_id: string; turn: Turn }> {
  const res = await fetch(`${API}/api/turns/stream`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    signal,
  });
  if (!res.ok) {
    const e = await res.json();
    throw new Error(e.error);
  }
  if (!res.body) throw new Error("Поток ответа недоступен");
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let pending = "";
  let result: { session_id: string; turn: Turn } | undefined;
  function line(value: string) {
    if (!value.trim()) return;
    const event = JSON.parse(value) as TraceEvent;
    onEvent(event);
    if (event.type === "error") throw new Error(String(event.data.error));
    if (event.type === "result")
      result = event.data as unknown as { session_id: string; turn: Turn };
  }
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      pending += decoder.decode(value, { stream: true });
      const lines = pending.split("\n");
      pending = lines.pop() || "";
      lines.forEach(line);
    }
    pending += decoder.decode();
    line(pending);
  } finally {
    reader.releaseLock();
  }
  if (!result)
    throw new Error(
      "Соединение завершилось без результата. Проверьте историю сессии.",
    );
  return result;
}
export async function transcribe(file: Blob, name: string) {
  const form = new FormData();
  form.append("audio", file, name);
  return api<{ text: string; stt_ms: number }>("/api/transcribe", {
    method: "POST",
    body: form,
  });
}
