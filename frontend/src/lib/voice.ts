/**
 * Browser voice I/O.
 *  - STT: Web Speech API (Chrome / Edge: ru-RU, kk-KZ). One utterance per start(): the browser
 *    stops on its own after a pause and delivers the final transcript.
 *  - TTS: ElevenLabs via /api/tts; speechSynthesis for keyless mock/fallback.
 *  - Recorder: MediaRecorder → base64 webm/opus for the real backend's STT.
 */
import type { ReplyLang } from "./contract";

/* eslint-disable @typescript-eslint/no-explicit-any */
type SR = any;

export function sttSupported(): boolean {
  if (typeof window === "undefined") return false;
  return !!((window as any).SpeechRecognition || (window as any).webkitSpeechRecognition);
}

export interface Listener {
  stop(): void;
  abort(): void;
}

const ERROR_TEXT: Record<string, string> = {
  "not-allowed": "Нет доступа к микрофону. Разрешите его в адресной строке и попробуйте ещё раз.",
  "service-not-allowed": "Браузер запретил распознавание речи. Откройте страницу в Chrome.",
  "audio-capture": "Микрофон не найден. Проверьте устройство ввода.",
  network: "Нет связи с сервисом распознавания. Проверьте интернет или напишите текстом.",
  "language-not-supported": "Этот язык не поддерживается распознаванием в вашем браузере.",
};

export function startListening(opts: {
  lang: "ru-RU" | "kk-KZ";
  onInterim(text: string): void;
  onFinal(text: string, endedAt: number): void;
  onEmpty(): void;
  onError(msg: string): void;
  onEnd(): void;
}): Listener | null {
  const Ctor = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
  if (!Ctor) return null;
  const rec: SR = new Ctor();
  rec.lang = opts.lang;
  rec.interimResults = true;
  rec.continuous = false; // one phrase → auto-stop on silence
  rec.maxAlternatives = 1;
  let finalText = "";
  let interim = "";
  let stoppedAt = 0;
  let delivered = false;
  let errored = false;
  rec.onresult = (e: any) => {
    interim = "";
    for (let i = e.resultIndex; i < e.results.length; i++) {
      const r = e.results[i];
      if (r.isFinal) finalText += r[0].transcript + " ";
      else interim += r[0].transcript;
    }
    opts.onInterim((finalText + interim).trim());
  };
  rec.onerror = (e: any) => {
    const code = String(e?.error ?? "");
    if (code === "aborted" || code === "no-speech") return;
    errored = true;
    opts.onError(ERROR_TEXT[code] ?? `Ошибка распознавания: ${code}`);
  };
  rec.onend = () => {
    const text = (finalText + interim).trim();
    if (!delivered && !errored) {
      delivered = true;
      if (text) opts.onFinal(text, stoppedAt || Date.now());
      else opts.onEmpty();
    }
    opts.onEnd();
  };
  try {
    rec.start();
  } catch (err) {
    opts.onError("Не удалось запустить распознавание: " + String(err));
    return null;
  }
  return {
    stop() { stoppedAt = Date.now(); try { rec.stop(); } catch { /* noop */ } },
    abort() { delivered = true; try { rec.abort(); } catch { /* noop */ } },
  };
}

/* ------------------------------ TTS ------------------------------ */

let currentAudio: HTMLAudioElement | null = null;
let pendingTts: AbortController | null = null;
let disposeAudio: (() => void) | null = null;

export function stopSpeaking() {
  if (typeof window === "undefined") return;
  pendingTts?.abort();
  pendingTts = null;
  disposeAudio?.();
  disposeAudio = null;
  try { window.speechSynthesis?.cancel(); } catch { /* noop */ }
  if (currentAudio) { currentAudio.pause(); currentAudio = null; }
}

/** Fetch server-generated MP3 and play it; resolves at actual playback start.
 * Stopping/resetting cancels pending synthesis and releases the blob URL.
 */
export async function speakElevenLabs(text: string, lang: ReplyLang, opts?: { onStart?(): void; onEnd?(): void }): Promise<number> {
  stopSpeaking();
  const controller = new AbortController();
  pendingTts = controller;
  const t0 = performance.now();
  try {
    const response = await fetch("/api/tts", {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text, lang }),
      signal: AbortSignal.any([controller.signal, AbortSignal.timeout(35_000)]),
    });
    if (!response.ok) {
      const result = await response.json().catch(() => ({}));
      throw new Error(result.error ?? `TTS: HTTP ${response.status}`);
    }
    const blob = await response.blob();
    if (controller.signal.aborted) { opts?.onEnd?.(); return 0; }
    if (!blob.size || !blob.type.startsWith("audio/")) throw new Error("TTS вернул пустое или некорректное аудио");
    return await new Promise<number>((resolve, reject) => {
      const url = URL.createObjectURL(blob);
      const audio = new Audio(url);
      currentAudio = audio;
      let finished = false;
      const cleanup = () => {
        if (finished) return;
        finished = true;
        clearTimeout(timer);
        audio.pause();
        audio.onplaying = audio.onended = audio.onerror = null;
        URL.revokeObjectURL(url);
        if (currentAudio === audio) currentAudio = null;
        if (disposeAudio === cleanup) disposeAudio = null;
        if (pendingTts === controller) pendingTts = null;
        opts?.onEnd?.();
        resolve(0); // settle even if cancelled before playback begins
      };
      const fail = () => {
        reject(new Error("Не удалось воспроизвести озвучку. Проверьте разрешение на звук в браузере."));
        cleanup();
      };
      const timer = setTimeout(fail, 15_000);
      disposeAudio = cleanup;
      audio.onplaying = () => { clearTimeout(timer); opts?.onStart?.(); resolve(Math.round(performance.now() - t0)); };
      audio.onended = cleanup;
      audio.onerror = fail;
      audio.play().catch(fail);
    });
  } catch (error) {
    if (pendingTts === controller) pendingTts = null;
    if (controller.signal.aborted) { opts?.onEnd?.(); return 0; }
    throw error;
  }
}

function pickVoice(lang: ReplyLang): SpeechSynthesisVoice | undefined {
  const voices = window.speechSynthesis.getVoices();
  const want = lang === "kk" ? ["kk-KZ", "kk"] : ["ru-RU", "ru"];
  const female = /(milena|anna|alena|irina|katya|tatyana|svetlana|zhanar|aigul|female|женск)/i;
  for (const w of want) {
    const pool = voices.filter((x) => x.lang.toLowerCase().startsWith(w.toLowerCase()));
    const v = pool.find((x) => female.test(x.name)) ?? pool.find((x) => /google|premium|enhanced/i.test(x.name)) ?? pool[0];
    if (v) return v;
  }
  return voices.find((x) => x.lang.toLowerCase().startsWith("ru")) ?? voices[0];
}

export function speak(text: string, lang: ReplyLang, opts?: { onStart?(): void; onEnd?(): void }): Promise<number> {
  return new Promise((resolve) => {
    if (typeof window === "undefined" || !window.speechSynthesis) { resolve(0); return; }
    stopSpeaking();
    const u = new SpeechSynthesisUtterance(text);
    const t0 = performance.now();
    u.lang = lang === "kk" ? "kk-KZ" : "ru-RU";
    const v = pickVoice(lang);
    if (v) u.voice = v;
    u.rate = 1.02;
    let started = false;
    u.onstart = () => { started = true; opts?.onStart?.(); resolve(Math.round(performance.now() - t0)); };
    u.onend = () => { opts?.onEnd?.(); if (!started) resolve(0); };
    u.onerror = () => { opts?.onEnd?.(); resolve(0); };
    window.speechSynthesis.speak(u);
    setTimeout(() => { if (!started) resolve(Math.round(performance.now() - t0)); }, 1500);
  });
}

export function playBase64(audio_base64: string, mime = "audio/mpeg", opts?: { onStart?(): void; onEnd?(): void }): Promise<number> {
  return new Promise((resolve) => {
    stopSpeaking();
    const a = new Audio(`data:${mime};base64,${audio_base64}`);
    const t0 = performance.now();
    a.onplaying = () => { opts?.onStart?.(); resolve(Math.round(performance.now() - t0)); };
    a.onended = () => opts?.onEnd?.();
    a.onerror = () => { opts?.onEnd?.(); resolve(0); };
    currentAudio = a;
    a.play().catch(() => resolve(0));
  });
}

/* ------------------------------ recorder (real backend) ------------------------------ */

export interface Recorder {
  stop(): Promise<{ audio_base64: string; mime: string; endedAt: number }>;
}

export async function startRecording(): Promise<Recorder> {
  const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
  const mime = MediaRecorder.isTypeSupported("audio/webm;codecs=opus") ? "audio/webm;codecs=opus" : "audio/webm";
  const rec = new MediaRecorder(stream, { mimeType: mime });
  const chunks: Blob[] = [];
  rec.ondataavailable = (e) => { if (e.data.size) chunks.push(e.data); };
  rec.start(250);
  return {
    stop: () =>
      new Promise((resolve) => {
        rec.onstop = async () => {
          stream.getTracks().forEach((t) => t.stop());
          const blob = new Blob(chunks, { type: mime });
          const buf = await blob.arrayBuffer();
          let bin = "";
          const bytes = new Uint8Array(buf);
          for (let i = 0; i < bytes.length; i += 0x8000) bin += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
          resolve({ audio_base64: btoa(bin), mime, endedAt: Date.now() });
        };
        rec.stop();
      }),
  };
}

/* ------------------------------ server STT (fallback) ------------------------------ */

/** Send a recording to /api/stt (Next route → OpenAI). Resolves with the transcript. */
export async function transcribeOnServer(audio_base64: string, mime: string, language: "ru" | "kk"): Promise<{ text: string; ms: number }> {
  const bin = atob(audio_base64);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  const fd = new FormData();
  fd.append("file", new Blob([bytes], { type: mime }), "audio.webm");
  fd.append("language", language);
  const r = await fetch("/api/stt", { method: "POST", body: fd });
  const j = (await r.json().catch(() => ({}))) as { text?: string; ms?: number; error?: string };
  if (!r.ok) throw new Error(j.error ?? `STT: HTTP ${r.status}`);
  return { text: j.text ?? "", ms: j.ms ?? 0 };
}
