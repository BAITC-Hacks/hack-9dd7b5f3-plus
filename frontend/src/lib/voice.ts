/**
 * Browser voice I/O.
 *  - STT: Web Speech API (Chrome: ru-RU / kk-KZ), interim results for the live transcript.
 *  - TTS: speechSynthesis (mock mode) or <audio> from base64 (real backend).
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

export function startListening(opts: {
  lang: "ru-RU" | "kk-KZ";
  onInterim(text: string): void;
  onFinal(text: string, endedAt: number): void;
  onError(msg: string): void;
  onEnd(): void;
}): Listener | null {
  const Ctor = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
  if (!Ctor) return null;
  const rec: SR = new Ctor();
  rec.lang = opts.lang;
  rec.interimResults = true;
  rec.continuous = true;
  rec.maxAlternatives = 1;
  let finalText = "";
  let interim = "";
  let stoppedAt = 0;
  let delivered = false;
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
    if (e.error === "no-speech" || e.error === "aborted") return;
    opts.onError(String(e.error ?? "stt error"));
  };
  rec.onend = () => {
    const text = (finalText + interim).trim();
    if (!delivered && text) {
      delivered = true;
      opts.onFinal(text, stoppedAt || Date.now());
    }
    opts.onEnd();
  };
  try {
    rec.start();
  } catch (err) {
    opts.onError(String(err));
    return null;
  }
  return {
    stop() { stoppedAt = Date.now(); try { rec.stop(); } catch { /* noop */ } },
    abort() { delivered = true; try { rec.abort(); } catch { /* noop */ } },
  };
}

/* ------------------------------ TTS ------------------------------ */

let currentUtterance: SpeechSynthesisUtterance | null = null;
let currentAudio: HTMLAudioElement | null = null;

export function stopSpeaking() {
  if (typeof window === "undefined") return;
  try { window.speechSynthesis?.cancel(); } catch { /* noop */ }
  currentUtterance = null;
  if (currentAudio) { currentAudio.pause(); currentAudio = null; }
}

function pickVoice(lang: ReplyLang): SpeechSynthesisVoice | undefined {
  const voices = window.speechSynthesis.getVoices();
  const want = lang === "kk" ? ["kk-KZ", "kk"] : ["ru-RU", "ru"];
  // the dataset's bot speaks in the feminine first person ("отправила", "жібердім") → prefer a female voice
  const female = /(milena|anna|alena|irina|katya|tatyana|svetlana|zhanar|aigul|female|женск)/i;
  for (const w of want) {
    const pool = voices.filter((x) => x.lang.toLowerCase().startsWith(w.toLowerCase()));
    const v = pool.find((x) => female.test(x.name)) ?? pool.find((x) => /google|premium|enhanced/i.test(x.name)) ?? pool[0];
    if (v) return v;
  }
  // Kazakh voices are rare — fall back to Russian (still Cyrillic) rather than English
  return voices.find((x) => x.lang.toLowerCase().startsWith("ru")) ?? voices[0];
}

/** Speak with the browser; resolves with ms-to-first-audio (from call time). */
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
    currentUtterance = u;
    window.speechSynthesis.speak(u);
    // Safari sometimes never fires onstart for short texts
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
