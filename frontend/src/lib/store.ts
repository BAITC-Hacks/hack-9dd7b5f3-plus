"use client";
/**
 * Conversation store shared by the client page and the admin console
 * (module-level, subscribed via useSyncExternalStore — no provider needed).
 */
import { useSyncExternalStore } from "react";
import { createSession, DEFAULT_MODE, runTurn, type ApiMode } from "./api";
import type { ActionCall, Candidate, DialogState, LatencyMs, PolicyVerdict, ReplyLang, RouterDecision, Stage, Trace, TurnEvent } from "./contract";
import { newDialogState } from "./mock/engine";
import { playBase64, speak, speakElevenLabs, startListening, startRecording, stopSpeaking, sttSupported, transcribeOnServer, type Listener, type Recorder } from "./voice";

export interface Message {
  id: string;
  role: "client" | "bot";
  text: string;
  lang?: string;
  turn: number;
  streaming?: boolean;
}

export type Status = "idle" | "listening" | "thinking" | "speaking";

export interface LiveTurn {
  turn: number;
  t0: number;
  stage: Stage | "done";
  transcript: string;
  language?: string;
  parts?: string[];
  urgent?: boolean;
  candidates: Candidate[];
  candidatesHistory: Candidate[][];
  decision?: RouterDecision;
  verdict?: PolicyVerdict;
  actions: ActionCall[];
  responseText: string;
  latency: LatencyMs;
  events: TurnEvent[];
  error?: string;
}

export interface ConversationState {
  mode: ApiMode;
  sessionId: string | null;
  dialog: DialogState;
  messages: Message[];
  traces: Trace[];
  live: LiveTurn | null;
  status: Status;
  interim: string;
  sttLang: "ru-RU" | "kk-KZ";
  /** browser = Chrome Web Speech (Google); server = /api/stt (OpenAI). */
  sttProvider: "browser" | "server";
  tts: boolean;
  sttAvailable: boolean;
  error: string | null;
  notice: string | null; // transient hint ("ничего не расслышала")
}

let state: ConversationState = {
  mode: DEFAULT_MODE,
  sessionId: null,
  dialog: newDialogState("—"),
  messages: [],
  traces: [],
  live: null,
  status: "idle",
  interim: "",
  sttLang: "ru-RU",
  sttProvider: "browser",
  tts: true,
  sttAvailable: false,
  error: null,
  notice: null,
};

const listeners = new Set<() => void>();
function emit() { for (const l of listeners) l(); }
function set(patch: Partial<ConversationState> | ((s: ConversationState) => Partial<ConversationState>)) {
  const p = typeof patch === "function" ? patch(state) : patch;
  state = { ...state, ...p };
  emit();
}
function subscribe(l: () => void) { listeners.add(l); return () => { listeners.delete(l); }; }
function getSnapshot() { return state; }
const serverSnapshot = state;
function getServerSnapshot() { return serverSnapshot; }

export function useConversation(): ConversationState {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

/* ------------------------------ actions ------------------------------ */

let listener: Listener | null = null;
let recorder: Recorder | null = null;
let busy = false;
let msgSeq = 0;

export async function ensureSession(): Promise<string> {
  if (state.sessionId) return state.sessionId;
  const { session_id, state: dialog } = await createSession(state.mode);
  set({ sessionId: session_id, dialog, sttAvailable: sttSupported() });
  return session_id;
}

export function setMode(mode: ApiMode) {
  if (busy || state.status === "listening") return;
  stopSpeaking();
  set({ mode, sessionId: null, messages: [], traces: [], live: null, status: "idle", error: null, notice: null, dialog: newDialogState("—") });
}

export function setSttLang(lang: "ru-RU" | "kk-KZ") { set({ sttLang: lang }); }
export function setSttProvider(p: "browser" | "server") { listener?.abort(); listener = null; set({ sttProvider: p, error: null, notice: null, status: "idle", interim: "" }); }
export function setTts(on: boolean) { if (!on) stopSpeaking(); set({ tts: on }); }

export function resetConversation() {
  if (busy) return;
  stopSpeaking();
  listener?.abort();
  listener = null;
  set({ sessionId: null, messages: [], traces: [], live: null, status: "idle", interim: "", error: null, notice: null, dialog: newDialogState("—") });
}

/** Click once to start, click again (or just pause) to send. */
export async function toggleVoice() {
  if (state.status === "listening") { await stopVoice(); return; }
  await startVoice();
}

export async function startVoice() {
  if (busy || state.status === "listening") return;
  stopSpeaking();
  set({ error: null, notice: null, sttAvailable: sttSupported() });
  try {
    await ensureSession();
  } catch (e) {
    set({ error: "Не удалось создать сессию: " + String(e) });
    return;
  }
  if (state.mode === "real" || state.sttProvider === "server") {
    try {
      recorder = await startRecording();
      set({ status: "listening", interim: "", notice: "Говорите. Нажмите на микрофон ещё раз, когда закончите." });
    } catch {
      set({ error: "Нет доступа к микрофону. Разрешите его в адресной строке." });
    }
    return;
  }
  if (!sttSupported()) {
    set({ error: "Распознавание речи работает в Chrome или Edge. Здесь можно написать текстом." });
    return;
  }
  listener = startListening({
    lang: state.sttLang,
    onInterim: (t) => set({ interim: t }),
    onFinal: (text, endedAt) => { void sendText(text, endedAt); },
    onEmpty: () => set({ notice: "Ничего не расслышала. Нажмите на микрофон и скажите ещё раз." }),
    onError: (m) => {
      if (/сервисом распознавания/.test(m)) {
        // Chrome could not reach Google's speech service → switch to server STT
        set({ sttProvider: "server", status: "idle", interim: "", error: null, notice: "Браузер не дотянулся до сервиса распознавания Google. Переключилась на серверное распознавание — нажмите на микрофон ещё раз." });
        return;
      }
      set({ error: m, status: "idle", interim: "" });
    },
    onEnd: () => { listener = null; set((s) => ({ status: s.status === "listening" ? "idle" : s.status, interim: "" })); },
  });
  if (!listener) {
    set({ error: "Не удалось включить микрофон. Попробуйте в Chrome.", status: "idle" });
    return;
  }
  set({ status: "listening", interim: "" });
}

export async function stopVoice() {
  if (recorder) {
    const r = recorder; recorder = null;
    set({ status: "thinking", notice: null });
    const { audio_base64, mime, endedAt } = await r.stop();
    if (state.mode === "real") {
      await runOne({ audio_base64, audio_mime: mime, client_t0: endedAt });
      return;
    }
    try {
      const { text } = await transcribeOnServer(audio_base64, mime, state.sttLang === "kk-KZ" ? "kk" : "ru");
      if (!text) { set({ status: "idle", notice: "Ничего не расслышала. Попробуйте ещё раз." }); return; }
      set({ status: "idle" });
      await sendText(text, endedAt);
    } catch (e) {
      set({ status: "idle", error: String(e instanceof Error ? e.message : e) });
    }
    return;
  }
  listener?.stop();
}

export async function sendText(text: string, endedAt = Date.now()) {
  const t = text.trim();
  if (!t) return;
  await runOne({ text: t, client_t0: endedAt });
}

async function runOne(req: { text?: string; audio_base64?: string; audio_mime?: string; client_t0: number }) {
  if (busy) return;
  busy = true;
  stopSpeaking();
  try {
    const session_id = await ensureSession();
    const turn = state.dialog.turn + 1;
    const clientMsgId = `m${++msgSeq}`;
    const botMsgId = `m${++msgSeq}`;
    const live: LiveTurn = { turn, t0: req.client_t0, stage: "stt", transcript: req.text ?? "", candidates: [], candidatesHistory: [], actions: [], responseText: "", latency: {}, events: [] };
    set((s) => ({
      status: "thinking",
      interim: "",
      error: null,
      notice: null,
      live,
      messages: [...s.messages, { id: clientMsgId, role: "client", text: req.text ?? "…", turn }],
    }));

    const patchLive = (p: Partial<LiveTurn> | ((l: LiveTurn) => Partial<LiveTurn>)) =>
      set((s) => (s.live ? { live: { ...s.live, ...(typeof p === "function" ? p(s.live) : p) } } : {}));

    let responseLang: ReplyLang = state.dialog.language;
    let finalText = "";
    let ttsPayload: { audio_base64?: string; mime?: string; browser_tts?: boolean } | null = null;
    let trace: Trace | null = null;

    try {
      for await (const ev of runTurn(state.mode, { session_id, ...req })) {
        patchLive((l) => ({ events: [...l.events, ev] }));
        switch (ev.type) {
          case "stt.partial":
            patchLive({ transcript: ev.text });
            break;
          case "stt.final":
            patchLive({ transcript: ev.text, language: ev.language, stage: "triage", latency: { ...live.latency, stt: ev.ms } });
            set((s) => ({ messages: s.messages.map((m) => (m.id === clientMsgId ? { ...m, text: ev.text, lang: ev.language } : m)) }));
            break;
          case "triage":
            patchLive((l) => ({ stage: "router", parts: ev.parts, urgent: ev.urgent, language: ev.language, latency: { ...l.latency, triage: ev.ms } }));
            break;
          case "router.candidates":
            patchLive((l) => ({ candidates: ev.candidates, candidatesHistory: [...l.candidatesHistory, ev.candidates] }));
            break;
          case "router.decision":
            patchLive((l) => ({ stage: "policy", decision: ev.decision, candidates: l.candidates.length ? l.candidates : ev.decision.scenarios.map((s) => ({ scenario_id: s.scenario_id, confidence: s.confidence })), latency: { ...l.latency, router: ev.ms } }));
            break;
          case "policy":
            patchLive((l) => ({ stage: "executor", verdict: ev.verdict, latency: { ...l.latency, policy: ev.ms } }));
            break;
          case "action":
            patchLive((l) => ({ actions: [...l.actions, ev.call] }));
            break;
          case "state":
            set({ dialog: ev.state });
            break;
          case "response.delta":
            patchLive((l) => ({ stage: "response", responseText: l.responseText + ev.text }));
            set((s) => {
              const exists = s.messages.some((m) => m.id === botMsgId);
              const msgs = exists
                ? s.messages.map((m) => (m.id === botMsgId ? { ...m, text: m.text + ev.text } : m))
                : [...s.messages, { id: botMsgId, role: "bot" as const, text: ev.text, turn, streaming: true }];
              return { messages: msgs };
            });
            break;
          case "response.final":
            finalText = ev.text;
            responseLang = ev.language;
            patchLive((l) => ({ stage: "tts_first_audio", responseText: ev.text, latency: { ...l.latency, response: ev.ms } }));
            set((s) => {
              const exists = s.messages.some((m) => m.id === botMsgId);
              return { messages: exists ? s.messages.map((m) => (m.id === botMsgId ? { ...m, text: ev.text, lang: ev.language, streaming: false } : m)) : [...s.messages, { id: botMsgId, role: "bot" as const, text: ev.text, lang: ev.language, turn }] };
            });
            break;
          case "tts.audio":
            ttsPayload = ev;
            break;
          case "turn.done":
            trace = ev.trace;
            patchLive((l) => ({ stage: "done", latency: { ...l.latency, ...ev.latency_ms } }));
            break;
          case "error":
            patchLive({ error: ev.message });
            set({ error: ev.message });
            break;
        }
      }
    } catch (e) {
      set({ error: String(e), status: "idle" });
    }

    // TTS + end-to-end latency (end of speech → first audio)
    let firstAudioMs = 0;
    if (state.tts && finalText) {
      set({ status: "speaking" });
      const onEnd = () => set((s) => (s.status === "speaking" ? { status: "idle" } : {}));
      if (ttsPayload?.audio_base64) firstAudioMs = await playBase64(ttsPayload.audio_base64, ttsPayload.mime, { onEnd });
      else if (state.mode !== "mock") {
        try {
          firstAudioMs = await speakElevenLabs(finalText, responseLang, { onEnd });
        } catch (error) {
          set({ notice: `${error instanceof Error ? error.message : "Ошибка ElevenLabs"} Использую голос браузера.`, status: "speaking" });
          firstAudioMs = await speak(finalText, responseLang, { onEnd });
        }
      } else firstAudioMs = await speak(finalText, responseLang, { onEnd });
    } else {
      set({ status: "idle" });
    }
    const e2e = Math.max(0, Date.now() - req.client_t0);
    if (trace) {
      const t: Trace = trace;
      const latency = { ...t.latency_ms, tts_first_audio: firstAudioMs, total: e2e };
      const done: Trace = { ...t, latency_ms: latency };
      set((s) => ({ traces: [...s.traces, done], live: s.live ? { ...s.live, latency } : s.live }));
    }
  } catch (e) {
    set({ error: e instanceof Error ? e.message : String(e), status: "idle" });
  } finally {
    busy = false;
  }
}
