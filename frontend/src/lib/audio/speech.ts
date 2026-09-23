// Browser speech APIs (keyless mode): speechSynthesis for TTS and
// (webkit)SpeechRecognition for STT. Minimal local typings — the recognition API
// is not in lib.dom and only Chromium ships it.

interface RecognitionAlternativeLike {
  transcript: string;
  confidence: number;
}
interface RecognitionResultLike {
  isFinal: boolean;
  length: number;
  [index: number]: RecognitionAlternativeLike;
}
interface RecognitionEventLike {
  resultIndex: number;
  results: { length: number; [index: number]: RecognitionResultLike };
}
interface RecognitionErrorLike {
  error: string;
  message?: string;
}
interface RecognitionLike {
  lang: string;
  continuous: boolean;
  interimResults: boolean;
  maxAlternatives: number;
  onstart: (() => void) | null;
  onresult: ((e: RecognitionEventLike) => void) | null;
  onerror: ((e: RecognitionErrorLike) => void) | null;
  onend: (() => void) | null;
  start(): void;
  stop(): void;
  abort(): void;
}
type RecognitionCtor = new () => RecognitionLike;

function recognitionCtor(): RecognitionCtor | null {
  if (typeof window === "undefined") return null;
  const w = window as unknown as { SpeechRecognition?: RecognitionCtor; webkitSpeechRecognition?: RecognitionCtor };
  return w.SpeechRecognition ?? w.webkitSpeechRecognition ?? null;
}

export interface RecognizerHandlers {
  onStart?: () => void;
  onPartial?: (text: string) => void;
  onFinal?: (text: string) => void;
  onError?: (error: string) => void;
  onEnd?: () => void;
}

export class BrowserRecognizer {
  private rec: RecognitionLike | null = null;
  private handlers: RecognizerHandlers;
  active = false;

  constructor(handlers: RecognizerHandlers) {
    this.handlers = handlers;
  }

  static available(): boolean {
    return recognitionCtor() !== null;
  }

  start(lang: string): boolean {
    const Ctor = recognitionCtor();
    if (!Ctor) return false;
    if (this.active) return true;
    let rec: RecognitionLike;
    try {
      rec = new Ctor();
    } catch {
      return false;
    }
    rec.lang = lang;
    rec.continuous = false;
    rec.interimResults = true;
    rec.maxAlternatives = 1;
    let finalText = "";
    rec.onstart = () => {
      this.active = true;
      this.handlers.onStart?.();
    };
    rec.onresult = (e) => {
      let interim = "";
      for (let i = e.resultIndex; i < e.results.length; i++) {
        const r = e.results[i];
        const text = r[0]?.transcript ?? "";
        if (r.isFinal) finalText += text;
        else interim += text;
      }
      if (finalText.trim()) {
        const t = finalText.trim();
        finalText = "";
        this.handlers.onFinal?.(t);
      } else if (interim) {
        this.handlers.onPartial?.(interim);
      }
    };
    rec.onerror = (e) => {
      this.handlers.onError?.(e.error || "unknown");
    };
    rec.onend = () => {
      this.active = false;
      this.rec = null;
      this.handlers.onEnd?.();
    };
    this.rec = rec;
    try {
      rec.start();
    } catch (e) {
      this.rec = null;
      this.active = false;
      this.handlers.onError?.(e instanceof Error ? e.message : "start failed");
      return false;
    }
    return true;
  }

  /** Ends the current utterance; a final result (if any) is delivered before onEnd. */
  stop(): void {
    try {
      this.rec?.stop();
    } catch {
      // ignore
    }
  }

  abort(): void {
    const rec = this.rec;
    this.rec = null;
    this.active = false;
    if (!rec) return;
    rec.onresult = null;
    rec.onerror = null;
    rec.onend = null;
    try {
      rec.abort();
    } catch {
      // ignore
    }
  }
}

// ---------------------------------------------------------------- synthesis

export function browserTtsAvailable(): boolean {
  return typeof window !== "undefined" && "speechSynthesis" in window && typeof SpeechSynthesisUtterance !== "undefined";
}

let voicesWarm = false;
function voices(): SpeechSynthesisVoice[] {
  if (!browserTtsAvailable()) return [];
  const list = window.speechSynthesis.getVoices();
  if (!voicesWarm) {
    voicesWarm = true;
    // Chrome loads voices asynchronously; touching the list once triggers it.
    window.speechSynthesis.addEventListener?.("voiceschanged", () => undefined, { once: true });
  }
  return list;
}

/** Prefer a voice for the reply language; Kazakh falls back to Russian (Cyrillic). */
export function pickVoice(lang: string): SpeechSynthesisVoice | null {
  const list = voices();
  if (list.length === 0) return null;
  const want = lang.toLowerCase().startsWith("kk") ? ["kk", "ru"] : ["ru"];
  for (const w of want) {
    const local = list.find((v) => v.lang.toLowerCase().startsWith(w) && v.localService);
    if (local) return local;
    const any = list.find((v) => v.lang.toLowerCase().startsWith(w));
    if (any) return any;
  }
  return null;
}

export interface SpeakHandlers {
  onStart?: () => void;
  onEnd?: () => void;
}

export function speakSentence(text: string, lang: string, handlers: SpeakHandlers = {}): boolean {
  if (!browserTtsAvailable() || !text.trim()) return false;
  try {
    const u = new SpeechSynthesisUtterance(text);
    u.lang = lang.toLowerCase().startsWith("kk") ? "kk-KZ" : "ru-RU";
    const v = pickVoice(lang);
    if (v) u.voice = v;
    u.rate = 1.05;
    let ended = false;
    const end = () => {
      if (ended) return;
      ended = true;
      handlers.onEnd?.();
    };
    u.onstart = () => handlers.onStart?.();
    u.onend = end;
    u.onerror = end;
    window.speechSynthesis.speak(u);
    return true;
  } catch {
    return false;
  }
}

export function cancelSpeech(): void {
  if (!browserTtsAvailable()) return;
  try {
    window.speechSynthesis.cancel();
  } catch {
    // ignore
  }
}
