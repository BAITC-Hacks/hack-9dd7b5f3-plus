import { api, errorMessage, wsUrl } from "./api";
import { MicCapture, type MicChunk } from "./audio/mic";
import { EnergyVAD } from "./audio/vad";
import { PCMPlayer } from "./audio/player";
import { BrowserRecognizer, browserTtsAvailable, cancelSpeech, speakSentence } from "./audio/speech";
import type {
  ActionRecord,
  AppConfig,
  ErrorData,
  Fact,
  FastPathCheck,
  Handoff,
  LLMStartData,
  LiveTrace,
  Path,
  ReplyDeltaData,
  ReplyDoneData,
  RetrievalData,
  RouteData,
  SessionEventData,
  SessionState,
  ShadowData,
  Signals,
  SpeakData,
  STTErrorData,
  STTFinalData,
  Trace,
  TraceView,
  TTSFirstByteData,
  TurnErrorData,
  TurnResponse,
  TurnStartData,
  VREvent,
} from "./types";

export type MicMode = "auto" | "ptt";
export type SttSource = "server" | "browser";
export type MicState = "idle" | "listening" | "thinking" | "speaking";
export type WsStatus = "idle" | "connecting" | "open" | "closed" | "error";
export type BrowserLang = "ru-RU" | "kk-KZ";

export interface TurnEntry extends TraceView {
  speak: string[];
  llmStream: string;
  client: { t0?: number; firstAudio?: number; e2e?: number };
  at: number;
}

export interface StatusLine {
  kind: "info" | "warn" | "error";
  text: string;
}

export interface CallState {
  sessionId: string | null;
  config: AppConfig | null;
  wsStatus: WsStatus;
  turns: TurnEntry[];
  selectedTurn: number | null;
  micState: MicState;
  armed: boolean;
  inSpeech: boolean;
  partial: string;
  mode: MicMode;
  sttSource: SttSource;
  browserLang: BrowserLang;
  browserSttAvailable: boolean;
  browserTtsAvailable: boolean;
  serverSttAvailable: boolean;
  micSupported: boolean;
  muted: boolean;
  bargeIn: boolean;
  sensitivity: number;
  status: StatusLine | null;
  sessionState: SessionState | null;
  audioSampleRate: number;
  restFallback: boolean;
}

const initialState: CallState = {
  sessionId: null,
  config: null,
  wsStatus: "idle",
  turns: [],
  selectedTurn: null,
  micState: "idle",
  armed: false,
  inSpeech: false,
  partial: "",
  mode: "auto",
  sttSource: "browser",
  browserLang: "ru-RU",
  browserSttAvailable: false,
  browserTtsAvailable: false,
  serverSttAvailable: false,
  micSupported: false,
  muted: false,
  bargeIn: false,
  sensitivity: 0.5,
  status: null,
  sessionState: null,
  audioSampleRate: 24000,
  restFallback: false,
};

const CHUNK_MS = 100;
const PREROLL_CHUNKS = 5;
const THINK_TIMEOUT_MS = 45_000;
const MAX_TURNS = 200;

type Listener = () => void;

function emptyTrace(): LiveTrace {
  return { timings: {} };
}

function newEntry(turn: number, transcript: string, source: string, at: number): TurnEntry {
  return {
    turn,
    transcript,
    source,
    reply: "",
    replyDone: false,
    trace: emptyTrace(),
    done: false,
    speak: [],
    llmStream: "",
    client: {},
    at,
  };
}

/**
 * Owns one voice session: WebSocket, microphone + VAD, playback, browser
 * speech APIs and the per-turn trace assembly. Exposed to React through
 * `useSyncExternalStore` (subscribe/getSnapshot); the mic level has its own
 * tiny store so the meter does not re-render the whole page.
 */
export class CallController {
  private state: CallState = initialState;
  private listeners = new Set<Listener>();
  private levelListeners = new Set<Listener>();
  private level = 0;

  private ws: WebSocket | null = null;
  private wsGeneration = 0;
  private mic = new MicCapture();
  private vad = new EnergyVAD(initialState.sensitivity);
  private player = new PCMPlayer();
  private recognizer: BrowserRecognizer | null = null;
  private preroll: Int16Array[] = [];
  private pendingT0: number | null = null;
  private currentTurn = 0;
  private audioEnded = true;
  private synthPending = 0;
  private firstAudioSeen = false;
  private thinkTimer = 0;
  private recognizerRestart = 0;
  private speakWatchdog = 0;
  private sessionReq = 0;
  private mountTimer = 0;
  private mounted = false;
  private sttChosenByUser = false;

  constructor() {
    this.mic.onChunk = (c) => this.onChunk(c);
    this.player.onFirstAudio = (t) => this.onFirstAudio(t);
    this.player.onDrain = () => this.checkDrained();
  }

  // ---------------------------------------------------------------- store

  subscribe = (l: Listener): (() => void) => {
    this.listeners.add(l);
    return () => {
      this.listeners.delete(l);
    };
  };
  getSnapshot = (): CallState => this.state;
  getServerSnapshot = (): CallState => initialState;

  subscribeLevel = (l: Listener): (() => void) => {
    this.levelListeners.add(l);
    return () => {
      this.levelListeners.delete(l);
    };
  };
  getLevel = (): number => this.level;

  private set(patch: Partial<CallState>): void {
    this.state = { ...this.state, ...patch };
    for (const l of this.listeners) l();
  }

  private setLevel(v: number): void {
    const next = Math.round(Math.min(1, Math.max(0, v)) * 100) / 100;
    if (next === this.level) return;
    this.level = next;
    for (const l of this.levelListeners) l();
  }

  private setStatus(kind: StatusLine["kind"], text: string): void {
    this.set({ status: { kind, text } });
  }

  // ---------------------------------------------------------------- lifecycle

  mount(): void {
    if (this.mounted) return;
    this.mounted = true;
    this.set({
      browserSttAvailable: BrowserRecognizer.available(),
      browserTtsAvailable: browserTtsAvailable(),
      micSupported: MicCapture.supported(),
    });
    // Deferred so React's dev-only mount/unmount/mount does not open two sessions.
    this.mountTimer = window.setTimeout(() => {
      this.mountTimer = 0;
      if (!this.mounted) return;
      void this.loadConfig();
      void this.newSession();
    }, 40);
  }

  unmount(): void {
    this.mounted = false;
    if (this.mountTimer) {
      window.clearTimeout(this.mountTimer);
      this.mountTimer = 0;
    }
    this.disarm();
    this.stopMic();
    this.closeWs();
    this.stopPlayback();
    this.player.dispose();
    this.clearThinkTimer();
    this.clearSpeakWatchdog();
  }

  private async loadConfig(): Promise<void> {
    try {
      const cfg = await api.config();
      if (!this.mounted) return;
      this.applyConfig(cfg);
    } catch (e) {
      if (!this.mounted) return;
      this.setStatus("error", `Config unavailable: ${errorMessage(e)}`);
    }
  }

  private applyConfig(cfg: AppConfig): void {
    const provider = (cfg.stt?.provider ?? "").toLowerCase();
    const serverStt = provider !== "" && provider !== "mock" && provider !== "browser";
    const patch: Partial<CallState> = { config: cfg, serverSttAvailable: serverStt };
    if (!this.sttChosenByUser) {
      patch.sttSource = serverStt ? "server" : "browser";
    }
    if (cfg.tts_sample_rate) {
      patch.audioSampleRate = cfg.tts_sample_rate;
      this.player.configure(cfg.tts_sample_rate);
    }
    this.set(patch);
  }

  async newSession(): Promise<void> {
    const req = ++this.sessionReq;
    this.stopPlayback();
    this.closeWs();
    this.clearThinkTimer();
    this.clearSpeakWatchdog();
    this.pendingT0 = null;
    this.currentTurn = 0;
    this.set({
      turns: [],
      selectedTurn: null,
      partial: "",
      sessionId: null,
      sessionState: null,
      inSpeech: false,
      micState: this.state.armed ? "listening" : "idle",
      wsStatus: "connecting",
      status: null,
      restFallback: false,
    });
    try {
      const r = await api.createSession("web");
      if (!this.mounted || req !== this.sessionReq) return;
      this.set({ sessionId: r.session_id, sessionState: r.session });
      this.connect(r.session_id);
    } catch (e) {
      if (!this.mounted || req !== this.sessionReq) return;
      this.set({ wsStatus: "error" });
      this.setStatus("error", `Cannot create a session: ${errorMessage(e)}`);
    }
  }

  private connect(id: string): void {
    this.closeWs();
    const gen = ++this.wsGeneration;
    let ws: WebSocket;
    try {
      ws = new WebSocket(wsUrl(id));
    } catch (e) {
      this.set({ wsStatus: "error", restFallback: true });
      this.setStatus("error", `WebSocket failed: ${errorMessage(e)} — text turns use REST`);
      return;
    }
    ws.binaryType = "arraybuffer";
    this.ws = ws;
    ws.onopen = () => {
      if (gen !== this.wsGeneration) return;
      this.set({ wsStatus: "open", restFallback: false });
      this.send({ type: "config", voice: true });
    };
    ws.onmessage = (ev: MessageEvent) => {
      if (gen !== this.wsGeneration) return;
      if (typeof ev.data === "string") {
        let parsed: VREvent;
        try {
          parsed = JSON.parse(ev.data) as VREvent;
        } catch {
          return;
        }
        this.handleEvent(parsed);
      } else if (ev.data instanceof ArrayBuffer) {
        this.handleAudio(ev.data);
      }
    };
    ws.onerror = () => {
      if (gen !== this.wsGeneration) return;
      this.set({ wsStatus: "error", restFallback: true });
      this.setStatus("error", "WebSocket error — text turns fall back to REST, voice is unavailable");
    };
    ws.onclose = () => {
      if (gen !== this.wsGeneration) return;
      this.ws = null;
      this.set({ wsStatus: "closed", restFallback: true });
      if (this.state.micState === "thinking") this.finishTurnAudio();
    };
  }

  private closeWs(): void {
    this.wsGeneration++;
    const ws = this.ws;
    this.ws = null;
    if (ws) {
      try {
        ws.onopen = null;
        ws.onmessage = null;
        ws.onerror = null;
        ws.onclose = null;
        ws.close();
      } catch {
        // ignore
      }
    }
  }

  private send(msg: Record<string, unknown>): boolean {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      try {
        this.ws.send(JSON.stringify(msg));
        return true;
      } catch {
        return false;
      }
    }
    return false;
  }

  private sendBinary(pcm: Int16Array): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      try {
        this.ws.send(pcm);
      } catch {
        // dropped frame
      }
    }
  }

  // ---------------------------------------------------------------- settings

  setMode(mode: MicMode): void {
    if (mode === this.state.mode) return;
    if (this.state.armed) this.disarm();
    if (this.state.inSpeech) this.cancelUtterance();
    this.set({ mode });
  }

  setSttSource(src: SttSource): void {
    if (src === this.state.sttSource) return;
    this.sttChosenByUser = true;
    if (this.state.armed) this.disarm();
    if (this.state.inSpeech) this.cancelUtterance();
    this.stopMic();
    this.set({ sttSource: src, partial: "" });
  }

  setBrowserLang(lang: BrowserLang): void {
    this.set({ browserLang: lang });
    if (this.recognizer?.active) {
      this.recognizer.abort();
      this.set({ inSpeech: false, partial: "" });
      this.scheduleRecognizer(100);
    }
  }

  setMuted(muted: boolean): void {
    this.set({ muted });
    this.player.setMuted(muted);
    if (muted) {
      cancelSpeech();
      this.synthPending = 0;
      this.checkDrained();
    }
  }

  setBargeIn(on: boolean): void {
    this.set({ bargeIn: on });
    if (on) this.ensureRecognizing();
  }

  setSensitivity(v: number): void {
    const s = Math.min(1, Math.max(0, v));
    this.vad.sensitivity = s;
    this.set({ sensitivity: s });
  }

  selectTurn(turn: number | null): void {
    this.set({ selectedTurn: turn });
  }

  dismissStatus(): void {
    this.set({ status: null });
  }

  // ---------------------------------------------------------------- text turns

  async sendText(text: string, source: "text" | "browser_stt" = "text"): Promise<void> {
    const t = text.trim();
    if (!t) return;
    void this.player.ensure();
    this.interruptIfSpeaking();
    this.pendingT0 = performance.now();
    this.beginThinking();
    if (this.send({ type: "text", text: t, source, t: Date.now() })) return;
    await this.restTurn(t, source);
  }

  private async restTurn(text: string, source: string): Promise<void> {
    const id = this.state.sessionId;
    if (!id) {
      this.setStatus("error", "No session — press New session");
      this.finishTurnAudio();
      return;
    }
    const at = Date.now();
    try {
      const r = await api.turn(id, { text, source, voice: false });
      if (!this.mounted) return;
      this.applyRestTurn(r, at);
    } catch (e) {
      if (!this.mounted) return;
      this.setStatus("error", `Turn failed: ${errorMessage(e)}`);
      this.finishTurnAudio();
    }
  }

  private applyRestTurn(r: TurnResponse, at: number): void {
    const tr = r.trace;
    const entry = newEntry(r.turn, tr?.input?.transcript ?? "", tr?.input?.source ?? "text", at);
    entry.reply = r.reply ?? "";
    entry.replyDone = true;
    entry.done = true;
    entry.trace = tr ? { ...tr } : { ...emptyTrace(), timings: r.timings ?? {} };
    entry.client.t0 = this.pendingT0 ?? undefined;
    this.pendingT0 = null;
    if (r.error) entry.error = r.error;
    this.currentTurn = r.turn;
    this.pushEntry(entry);
    if (tr?.state) this.set({ sessionState: tr.state });
    this.clearThinkTimer();
    // No audio stream over REST: speak the reply with the browser voice.
    this.audioEnded = false;
    this.synthPending = 0;
    this.firstAudioSeen = false;
    const lang = tr?.reply?.lang || r.language || "ru";
    const sentences = (entry.reply || "").split(/(?<=[.!?…])\s+/).filter((s) => s.trim());
    if (this.state.muted || sentences.length === 0 || !this.state.browserTtsAvailable) {
      this.audioEnded = true;
      this.finishTurnAudio();
      return;
    }
    for (const s of sentences) this.speakNow(s, lang);
    this.audioEnded = true;
    this.checkDrained();
  }

  // ---------------------------------------------------------------- mic / VAD / PTT

  async toggleArmed(): Promise<void> {
    if (this.state.armed) this.disarm();
    else await this.arm();
  }

  private async arm(): Promise<void> {
    void this.player.ensure();
    if (this.state.sttSource === "browser") {
      if (!this.state.browserSttAvailable) {
        this.setStatus("error", "Browser speech recognition is not available here (use Chrome) — type instead");
        return;
      }
      this.set({ armed: true, micState: this.state.micState === "idle" ? "listening" : this.state.micState });
      void this.startMicForLevel();
      this.ensureRecognizing();
      return;
    }
    if (!this.state.serverSttAvailable) {
      this.setStatus("warn", "Server STT is not configured — switching to browser STT");
      this.set({ sttSource: "browser" });
      await this.arm();
      return;
    }
    try {
      await this.mic.start();
    } catch (e) {
      this.setStatus("error", `Microphone: ${errorMessage(e)}`);
      return;
    }
    this.vad.reset();
    this.preroll = [];
    this.set({ armed: true, micState: this.state.micState === "idle" ? "listening" : this.state.micState, status: null });
  }

  disarm(): void {
    if (this.recognizerRestart) {
      window.clearTimeout(this.recognizerRestart);
      this.recognizerRestart = 0;
    }
    this.recognizer?.abort();
    const patch: Partial<CallState> = { armed: false, inSpeech: false, partial: "" };
    if (this.state.micState === "listening") patch.micState = "idle";
    this.set(patch);
    this.setLevel(0);
    this.vad.reset();
    this.preroll = [];
  }

  async pttStart(): Promise<void> {
    if (this.state.inSpeech || this.state.mode !== "ptt") return;
    void this.player.ensure();
    this.interruptIfSpeaking();
    if (this.state.sttSource === "browser") {
      if (!this.state.browserSttAvailable) {
        this.setStatus("error", "Browser speech recognition is not available here (use Chrome) — type instead");
        return;
      }
      void this.startMicForLevel();
      this.set({ inSpeech: true, micState: "listening", partial: "" });
      this.startRecognition();
      return;
    }
    if (!this.state.serverSttAvailable) {
      this.setStatus("warn", "Server STT is not configured — switching to browser STT");
      this.set({ sttSource: "browser" });
      await this.pttStart();
      return;
    }
    try {
      await this.mic.start();
    } catch (e) {
      this.setStatus("error", `Microphone: ${errorMessage(e)}`);
      return;
    }
    this.beginUtterance();
  }

  pttEnd(): void {
    if (!this.state.inSpeech || this.state.mode !== "ptt") return;
    if (this.state.sttSource === "browser") {
      this.recognizer?.stop();
      this.set({ inSpeech: false });
      return;
    }
    this.endUtterance();
  }

  /** Stop whatever is happening: playback, capture, the running turn. */
  cancel(): void {
    this.send({ type: "cancel" });
    this.stopPlayback();
    if (this.state.inSpeech) this.cancelUtterance();
    this.clearThinkTimer();
    this.set({ micState: this.state.armed ? "listening" : "idle", partial: "" });
    this.ensureRecognizing();
  }

  private async startMicForLevel(): Promise<void> {
    if (!this.state.micSupported) return;
    try {
      await this.mic.start();
    } catch {
      // level meter only — recognition works without it
    }
  }

  private stopMic(): void {
    this.mic.stop();
    this.setLevel(0);
  }

  private onChunk(c: MicChunk): void {
    const st = this.state;
    this.setLevel(Math.min(1, c.rms * 7));
    if (st.sttSource === "browser") return;
    if (st.mode === "ptt") {
      if (st.inSpeech) this.sendBinary(c.pcm);
      return;
    }
    if (!st.armed) return;
    const busy = st.micState === "thinking" || st.micState === "speaking";
    if (busy && (!st.bargeIn || st.micState === "thinking")) {
      // ignore the robot's own voice / wait for the answer
      this.vad.observe(c.rms);
      this.preroll = [];
      return;
    }
    const ev = this.vad.feed(c.rms, CHUNK_MS);
    if (!st.inSpeech) {
      this.preroll.push(c.pcm);
      if (this.preroll.length > PREROLL_CHUNKS) this.preroll.shift();
    }
    if (ev === "start") {
      this.beginUtterance();
      for (const p of this.preroll) this.sendBinary(p);
      this.preroll = [];
      return;
    }
    if (this.state.inSpeech) this.sendBinary(c.pcm);
    if (ev === "end") this.endUtterance();
    else if (ev === "abort") this.abortUtterance();
  }

  private beginUtterance(): void {
    this.interruptIfSpeaking();
    this.send({ type: "speech_start", t: Date.now() });
    this.set({ inSpeech: true, micState: "listening", partial: "" });
  }

  private endUtterance(): void {
    this.pendingT0 = performance.now();
    this.send({ type: "speech_end", t: Date.now() });
    this.set({ inSpeech: false });
    this.beginThinking();
  }

  private abortUtterance(): void {
    this.set({ inSpeech: false });
    this.setStatus("info", "Too short — ignored");
  }

  private cancelUtterance(): void {
    // The server drops the buffered audio on the next speech_start; nothing to send.
    this.recognizer?.abort();
    this.set({ inSpeech: false, partial: "" });
    this.vad.reset();
    this.preroll = [];
  }

  // ---------------------------------------------------------------- browser STT

  private ensureRecognizing(): void {
    const st = this.state;
    if (st.sttSource !== "browser" || !st.armed || st.mode !== "auto") return;
    const canListen = st.micState === "listening" || (st.bargeIn && st.micState === "speaking");
    if (!canListen) return;
    if (this.recognizer?.active) return;
    this.startRecognition();
  }

  private scheduleRecognizer(delay: number): void {
    if (this.recognizerRestart) window.clearTimeout(this.recognizerRestart);
    this.recognizerRestart = window.setTimeout(() => {
      this.recognizerRestart = 0;
      this.ensureRecognizing();
    }, delay);
  }

  private startRecognition(): void {
    if (!this.recognizer) {
      this.recognizer = new BrowserRecognizer({
        onStart: () => {
          if (this.state.mode === "auto") this.set({ inSpeech: true });
        },
        onPartial: (text) => this.set({ partial: text }),
        onFinal: (text) => {
          this.set({ partial: "", inSpeech: false });
          void this.sendText(text, "browser_stt");
        },
        onError: (err) => {
          if (err === "no-speech" || err === "aborted") return;
          if (err === "not-allowed" || err === "service-not-allowed") {
            this.setStatus("error", "Browser STT: microphone permission denied");
            this.disarm();
            return;
          }
          if (err === "network") {
            this.setStatus("warn", "Browser STT: network error (Chrome's recognizer needs internet)");
            return;
          }
          this.setStatus("warn", `Browser STT: ${err}`);
        },
        onEnd: () => {
          this.set({ inSpeech: false, partial: "" });
          if (this.state.armed && this.state.mode === "auto") this.scheduleRecognizer(250);
        },
      });
    }
    if (!this.recognizer.start(this.state.browserLang)) {
      this.setStatus("error", "Browser STT could not start");
    }
  }

  // ---------------------------------------------------------------- thinking / speaking

  private beginThinking(): void {
    this.clearThinkTimer();
    this.audioEnded = false;
    this.synthPending = 0;
    this.firstAudioSeen = false;
    this.player.beginReply();
    this.set({ micState: "thinking" });
    this.thinkTimer = window.setTimeout(() => {
      this.thinkTimer = 0;
      if (this.state.micState === "thinking") {
        this.setStatus("error", "No answer from the server (timeout)");
        this.finishTurnAudio();
      }
    }, THINK_TIMEOUT_MS);
  }

  private clearThinkTimer(): void {
    if (this.thinkTimer) {
      window.clearTimeout(this.thinkTimer);
      this.thinkTimer = 0;
    }
  }

  private interruptIfSpeaking(): void {
    if (this.state.micState === "speaking") {
      this.send({ type: "cancel" });
      this.stopPlayback();
    }
  }

  private stopPlayback(): void {
    this.player.stop();
    cancelSpeech();
    this.synthPending = 0;
    this.audioEnded = true;
  }

  private handleAudio(buf: ArrayBuffer): void {
    if (this.state.micState === "thinking" && !this.firstAudioSeen && this.state.muted) {
      this.onFirstAudio(performance.now());
    }
    if (this.state.muted) return;
    this.player.enqueue(buf);
  }

  private onFirstAudio(t: number): void {
    if (this.firstAudioSeen) return;
    this.firstAudioSeen = true;
    this.clearThinkTimer();
    const turn = this.currentTurn;
    this.updateTurn(turn, (e) => {
      const t0 = e.client.t0;
      return { ...e, client: { ...e.client, firstAudio: t, e2e: t0 !== undefined ? Math.max(0, Math.round(t - t0)) : undefined } };
    });
    if (this.state.micState === "thinking") this.set({ micState: "speaking" });
  }

  private speakNow(text: string, lang: string): void {
    this.synthPending++;
    const ok = speakSentence(text, lang, {
      onStart: () => this.onFirstAudio(performance.now()),
      onEnd: () => {
        this.synthPending = Math.max(0, this.synthPending - 1);
        this.checkDrained();
      },
    });
    if (!ok) {
      this.synthPending = Math.max(0, this.synthPending - 1);
      this.checkDrained();
    }
  }

  private handleSpeak(d: SpeakData): void {
    if (this.state.muted || !this.state.browserTtsAvailable) {
      if (!this.firstAudioSeen) this.onFirstAudio(performance.now());
      return;
    }
    this.speakNow(d.text, d.lang);
  }

  private checkDrained(): void {
    if (!this.audioEnded) return;
    if (this.synthPending > 0 || this.player.busy) return;
    this.finishTurnAudio();
  }

  /** speechSynthesis occasionally never fires onend (background tabs); do not stay "speaking" forever. */
  private armSpeakWatchdog(): void {
    this.clearSpeakWatchdog();
    if (this.state.micState !== "speaking" && this.state.micState !== "thinking") return;
    const turn = this.currentTurn;
    const chars = (this.state.turns.find((t) => t.turn === turn)?.speak ?? []).join(" ").length;
    const budget = Math.min(40_000, Math.max(4000, chars * 90));
    this.speakWatchdog = window.setTimeout(() => {
      this.speakWatchdog = 0;
      if (this.currentTurn !== turn) return;
      if (this.state.micState === "speaking" || this.state.micState === "thinking") {
        this.synthPending = 0;
        this.player.stop();
        this.finishTurnAudio();
      }
    }, budget);
  }

  private clearSpeakWatchdog(): void {
    if (this.speakWatchdog) {
      window.clearTimeout(this.speakWatchdog);
      this.speakWatchdog = 0;
    }
  }

  /** Reply finished (or failed): back to listening/idle. */
  private finishTurnAudio(): void {
    this.clearThinkTimer();
    this.clearSpeakWatchdog();
    this.audioEnded = true;
    const st = this.state;
    if (st.micState === "speaking" || st.micState === "thinking") {
      this.set({ micState: st.armed ? "listening" : "idle" });
    }
    this.vad.reset();
    this.preroll = [];
    this.ensureRecognizing();
  }

  // ---------------------------------------------------------------- turn bookkeeping

  private pushEntry(entry: TurnEntry): void {
    const turns = this.state.turns.filter((t) => t.turn !== entry.turn).concat(entry);
    turns.sort((a, b) => a.turn - b.turn);
    this.set({ turns: turns.length > MAX_TURNS ? turns.slice(turns.length - MAX_TURNS) : turns });
  }

  private updateTurn(turn: number, fn: (e: TurnEntry) => TurnEntry): void {
    const turns = this.state.turns;
    const idx = turns.findIndex((t) => t.turn === turn);
    if (idx < 0) return;
    const next = turns.slice();
    next[idx] = fn(turns[idx]);
    this.set({ turns: next });
  }

  private patchTrace(turn: number, fn: (t: LiveTrace) => LiveTrace): void {
    this.updateTurn(turn, (e) => ({ ...e, trace: fn(e.trace) }));
  }

  private turnOf(ev: VREvent): number {
    if (typeof ev.turn === "number" && ev.turn > 0) {
      if (!this.state.turns.some((t) => t.turn === ev.turn)) {
        this.pushEntry(newEntry(ev.turn, "", "", ev.t || Date.now()));
      }
      return ev.turn;
    }
    return this.currentTurn;
  }

  private provisionalPath(fp: FastPathCheck | undefined): Path {
    const cfg = this.state.config;
    if (!cfg) return "llm";
    if ((cfg.llm?.provider ?? "").toLowerCase() === "mock") return "mock";
    if (fp?.eligible && cfg.fast_path === "on") return "fast";
    return "llm";
  }

  // ---------------------------------------------------------------- events

  private handleEvent(ev: VREvent): void {
    switch (ev.type) {
      case "session": {
        const d = ev.data as SessionEventData;
        const patch: Partial<CallState> = {};
        if (d.session_id) patch.sessionId = d.session_id;
        if (d.state) patch.sessionState = d.state;
        if (d.audio_out?.sample_rate) {
          patch.audioSampleRate = d.audio_out.sample_rate;
          this.player.configure(d.audio_out.sample_rate);
        }
        this.set(patch);
        if (d.config) this.applyConfig({ ...d.config, tts_sample_rate: d.audio_out?.sample_rate });
        return;
      }
      case "turn_start": {
        const d = ev.data as TurnStartData;
        const turn = typeof ev.turn === "number" ? ev.turn : this.currentTurn + 1;
        this.currentTurn = turn;
        const entry = newEntry(turn, d.text ?? "", d.source ?? "", ev.t || Date.now());
        entry.client.t0 = this.pendingT0 ?? undefined;
        this.pendingT0 = null;
        this.pushEntry(entry);
        this.clearSpeakWatchdog();
        this.set({ partial: "" });
        if (this.state.micState !== "thinking") {
          // a turn started server-side without our thinking state (e.g. REST from elsewhere)
          this.beginThinking();
        }
        return;
      }
      case "triage": {
        const sig = ev.data as Signals;
        this.patchTrace(this.turnOf(ev), (t) => ({
          ...t,
          triage: sig,
          language: { detected: sig.language, kk_share: sig.kk_share, reply: t.language?.reply ?? "" },
        }));
        return;
      }
      case "retrieval": {
        const d = ev.data as RetrievalData;
        this.patchTrace(this.turnOf(ev), (t) => ({
          ...t,
          retrieval: d.candidates ?? [],
          timings: { ...t.timings, retrieval: d.ms },
        }));
        return;
      }
      case "facts": {
        const d = ev.data as Fact[] | null;
        this.patchTrace(this.turnOf(ev), (t) => ({ ...t, facts: d ?? [] }));
        return;
      }
      case "fast_path": {
        const d = ev.data as FastPathCheck;
        this.patchTrace(this.turnOf(ev), (t) => ({ ...t, fast_path: d }));
        this.updateTurn(this.turnOf(ev), (e) => ({ ...e, provisionalPath: this.provisionalPath(d) }));
        return;
      }
      case "llm_start": {
        const d = ev.data as LLMStartData;
        this.updateTurn(this.turnOf(ev), (e) => ({
          ...e,
          llmStream: d.follow_up ? e.llmStream : "",
          trace: {
            ...e.trace,
            llm: { provider: e.trace.llm?.provider ?? "", model: d.model, ttft_ms: 0, total_ms: 0, messages: d.messages ?? undefined },
          },
        }));
        return;
      }
      case "llm_delta": {
        const d = ev.data as { text?: string };
        if (!d?.text) return;
        this.updateTurn(this.turnOf(ev), (e) => ({ ...e, llmStream: e.llmStream + d.text }));
        return;
      }
      case "llm_error": {
        const d = ev.data as ErrorData;
        this.patchTrace(this.turnOf(ev), (t) => ({ ...t, errors: [...(t.errors ?? []), `llm: ${d.error}`] }));
        return;
      }
      case "route": {
        const d = ev.data as RouteData;
        this.patchTrace(this.turnOf(ev), (t) => ({
          ...t,
          decision: d.decision,
          policy: d.verdict,
          timings: { ...t.timings, route: d.since_t0_ms, llm_route: d.route_ms },
          language: {
            detected: t.language?.detected ?? d.decision?.language ?? "",
            kk_share: t.language?.kk_share ?? 0,
            reply: d.language,
          },
        }));
        return;
      }
      case "reply_delta": {
        const d = ev.data as ReplyDeltaData;
        this.updateTurn(this.turnOf(ev), (e) => ({ ...e, reply: e.reply + (d.text ?? "") }));
        return;
      }
      case "reply_done": {
        const d = ev.data as ReplyDoneData;
        this.updateTurn(this.turnOf(ev), (e) => ({ ...e, reply: d.text ?? e.reply, replyDone: true }));
        return;
      }
      case "action": {
        const d = ev.data as ActionRecord;
        this.patchTrace(this.turnOf(ev), (t) => ({ ...t, actions: [...(t.actions ?? []), d] }));
        return;
      }
      case "handoff": {
        const d = ev.data as Handoff;
        this.patchTrace(this.turnOf(ev), (t) => ({
          ...t,
          decision: t.decision ? { ...t.decision, handoff: d } : t.decision,
        }));
        return;
      }
      case "speak": {
        const d = ev.data as SpeakData;
        this.updateTurn(this.turnOf(ev), (e) => ({ ...e, speak: [...e.speak, d.text] }));
        this.handleSpeak(d);
        return;
      }
      case "tts_first_byte": {
        const d = ev.data as TTSFirstByteData;
        this.patchTrace(this.turnOf(ev), (t) => ({
          ...t,
          timings: { ...t.timings, tts_first_byte: d.tts_ms, first_audio: d.first_audio_ms },
        }));
        return;
      }
      case "tts_sentence":
        return;
      case "tts_error": {
        const d = ev.data as ErrorData;
        this.patchTrace(this.turnOf(ev), (t) => ({ ...t, errors: [...(t.errors ?? []), `tts: ${d.error}`] }));
        return;
      }
      case "audio_end": {
        this.audioEnded = true;
        this.checkDrained();
        this.armSpeakWatchdog();
        return;
      }
      case "turn_done": {
        const tr = ev.data as Trace;
        const turn = typeof ev.turn === "number" ? ev.turn : tr.turn;
        this.updateTurn(this.turnOf({ ...ev, turn }), (e) => ({
          ...e,
          trace: { ...tr, shadow: tr.shadow ?? e.trace.shadow },
          done: true,
          reply: tr.reply?.text || e.reply,
          replyDone: true,
          transcript: tr.input?.transcript || e.transcript,
          source: tr.input?.source || e.source,
          provisionalPath: undefined,
        }));
        if (tr.state) this.set({ sessionState: tr.state });
        if (!this.firstAudioSeen && this.audioEnded) this.finishTurnAudio();
        return;
      }
      case "shadow": {
        const d = ev.data as ShadowData;
        this.patchTrace(this.turnOf(ev), (t) => ({ ...t, shadow: d.shadow }));
        return;
      }
      case "turn_error": {
        const d = ev.data as TurnErrorData;
        const msg = d.empty ? "Nothing heard — try again" : `Turn failed: ${d.error}`;
        this.setStatus(d.empty ? "warn" : "error", msg);
        if (typeof ev.turn === "number" && ev.turn > 0) {
          this.updateTurn(ev.turn, (e) => ({ ...e, error: d.error, done: true }));
        }
        this.finishTurnAudio();
        return;
      }
      case "error": {
        const d = ev.data as ErrorData;
        this.setStatus("error", d.error);
        if (typeof ev.turn === "number" && ev.turn > 0) {
          this.updateTurn(ev.turn, (e) => ({ ...e, error: d.error, done: true }));
        }
        this.finishTurnAudio();
        return;
      }
      case "stt_partial": {
        const d = ev.data as { text?: string };
        this.set({ partial: d.text ?? "" });
        return;
      }
      case "stt_start": {
        this.setStatus("info", "Transcribing…");
        return;
      }
      case "stt_final": {
        const d = ev.data as STTFinalData;
        this.set({ partial: "", status: null });
        if (!d.text) this.setStatus("warn", "Nothing heard — try again");
        return;
      }
      case "stt_error": {
        const d = ev.data as STTErrorData;
        this.setStatus(d.fallback ? "warn" : "error", `STT ${d.stage}: ${d.error}${d.fallback ? ` (falling back to ${d.fallback})` : ""}`);
        return;
      }
      case "stt_realtime":
      case "pong":
        return;
      default:
        return;
    }
  }
}
