import { API } from "./api";

// OpenAI PCM is signed little-endian 16-bit, mono, 24 kHz. Schedule chunks
// immediately; do not wait for the whole utterance to download or decode.
export async function playPCM(sessionID: string, turnID: string, context: AudioContext, signal: AbortSignal, onStart: (firstByteMS: number, playbackAt: number) => void) {
  await context.resume();
  const res = await fetch(`${API}/api/sessions/${sessionID}/turns/${turnID}/speech`, { signal });
  if (!res.ok) { const err = await res.json(); throw new Error(err.error || "Ошибка синтеза речи"); }
  if (!res.body) throw new Error("Нет аудиопотока");
  const reader = res.body.getReader(); let spare: number | undefined; let next = context.currentTime + 0.03; let first = true; const nodes: AudioBufferSourceNode[] = [];
  const stop = () => nodes.forEach(node => { try { node.stop(); } catch { /* already stopped */ } }); signal.addEventListener("abort", stop);
  try {
    for (;;) {
      const { done, value } = await reader.read(); if (done) break;
      const bytes = new Uint8Array(value.length + (spare === undefined ? 0 : 1)); let offset = 0;
      if (spare !== undefined) { bytes[0] = spare; offset = 1; } bytes.set(value, offset);
      spare = bytes.length % 2 ? bytes[bytes.length - 1] : undefined;
      const count = Math.floor(bytes.length / 2); if (!count) continue;
      const buffer = context.createBuffer(1, count, 24000); const output = buffer.getChannelData(0); const view = new DataView(bytes.buffer);
      for (let i = 0; i < count; i++) output[i] = view.getInt16(i * 2, true) / 32768;
      const node = context.createBufferSource(); node.buffer = buffer; node.connect(context.destination);
      next = Math.max(next, context.currentTime + 0.015); node.start(next); nodes.push(node);
      if (first) { first = false; const latency = context.baseLatency + (context.outputLatency || 0); onStart(Number(res.headers.get("X-TTS-First-Byte-MS") || 0), performance.now() + (next - context.currentTime + latency) * 1000); }
      next += buffer.duration;
    }
    if (first) throw new Error("Синтезатор вернул пустое аудио");
    await new Promise<void>((resolve, reject) => {
      const remaining = Math.max(0, (next - context.currentTime) * 1000);
      const timer = setTimeout(() => { signal.removeEventListener("abort", abort); resolve(); }, remaining);
      function abort() { clearTimeout(timer); reject(new DOMException("Aborted", "AbortError")); }
      signal.addEventListener("abort", abort, { once: true }); if (signal.aborted) abort();
    });
  } finally { signal.removeEventListener("abort", stop); if (signal.aborted) stop(); reader.releaseLock(); nodes.forEach(n => n.disconnect()); }
}

type RecognitionEvent = { results: { [index: number]: { isFinal: boolean; 0: { transcript: string } }; length: number }; resultIndex: number };
type Recognition = { lang: string; interimResults: boolean; continuous: boolean; onresult: ((e: RecognitionEvent) => void) | null; onerror: ((e: { error: string }) => void) | null; onend: (() => void) | null; start(): void; stop(): void; abort(): void };
type SpeechWindow = Window & { SpeechRecognition?: new () => Recognition; webkitSpeechRecognition?: new () => Recognition };
export function browserRecognition(language: string, onPartial: (text: string) => void, onFinal: (text: string, endAt: number, sttMS: number) => void, onError: (error: string) => void) {
  const w = window as SpeechWindow; const Constructor = w.SpeechRecognition || w.webkitSpeechRecognition;
  if (!Constructor) throw new Error("Браузер не поддерживает SpeechRecognition. Подключите OpenAI или используйте текст / аудиофайл.");
  const rec = new Constructor(); rec.lang = language; rec.interimResults = true; rec.continuous = false; let final = ""; let lastPartial = performance.now(); let canceled = false;
  rec.onresult = event => { let partial = ""; for (let i = 0; i < event.results.length; i++) { partial += event.results[i][0].transcript; if (event.results[i].isFinal) final += event.results[i][0].transcript; } lastPartial = performance.now(); onPartial(partial); };
  rec.onerror = e => { if (!canceled) onError(`Распознавание: ${e.error}. Проверьте разрешение микрофона.`); };
  // Browser recognition does not expose a reliable acoustic end timestamp.
  // NaN prevents this estimate from entering the end-of-speech latency metric.
  rec.onend = () => { if (!canceled) { if (final.trim()) onFinal(final.trim(), Number.NaN, performance.now() - lastPartial); else onError("Речь не распознана. Попробуйте ещё раз."); } };
  rec.start(); return { stop: () => rec.stop(), cancel: () => { canceled = true; rec.abort(); } };
}

export type Capture = { commit(): void; close(): void };
export async function realtimeCapture(onState: (state: string) => void, onPartial: (text: string) => void, onFinal: (text: string, endAt: number, sttMS: number) => void, onError: (error: string) => void): Promise<Capture> {
  const media = await navigator.mediaDevices.getUserMedia({ audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true } });
  const pc = new RTCPeerConnection(); const dc = pc.createDataChannel("oai-events"); const ctx = new AudioContext(); const analyser = ctx.createAnalyser(); analyser.fftSize = 1024; ctx.createMediaStreamSource(media).connect(analyser);
  media.getTracks().forEach(t => pc.addTrack(t, media));
  let closed = false; let committed = false; let speaking = false; let voicedFrames = 0; let lastVoice = performance.now(); let endAt = lastVoice; let timer: ReturnType<typeof setInterval> | undefined; let deadline: ReturnType<typeof setTimeout> | undefined; let partial = "";
  function close() { if (closed) return; closed = true; clearInterval(timer); clearTimeout(deadline); media.getTracks().forEach(t => t.stop()); dc.close(); pc.close(); void ctx.close(); }
  function fail(message: string) { close(); onError(message); }
  function commit() {
    if (closed || committed || dc.readyState !== "open") return;
    if (!speaking) { fail("Не удалось обнаружить речь. Попробуйте говорить ближе к микрофону."); return; }
    committed = true; endAt = lastVoice; clearInterval(timer); media.getTracks().forEach(t => { t.enabled = false; }); dc.send(JSON.stringify({ type: "input_audio_buffer.commit" })); onState("Распознавание");
    clearTimeout(deadline); deadline = setTimeout(() => fail("Истекло время ожидания транскрипта"), 20000);
  }
  dc.onmessage = message => {
    try {
      const event = JSON.parse(message.data);
      if (event.type === "error") { fail(event.error?.message || "Ошибка Realtime API"); return; }
      if (event.type === "conversation.item.input_audio_transcription.failed") { fail("Потоковое распознавание не удалось"); return; }
      if (event.type === "conversation.item.input_audio_transcription.delta") { partial += event.delta; onPartial(partial); }
      if (event.type === "conversation.item.input_audio_transcription.completed" && committed) { const text = String(event.transcript || "").trim(); const latency = performance.now() - endAt; close(); if (text) onFinal(text, endAt, latency); else onError("Пустой транскрипт"); }
    } catch { fail("Некорректное событие распознавания"); }
  };
  pc.onconnectionstatechange = () => { if (!closed && ["failed", "disconnected"].includes(pc.connectionState)) fail("Соединение с распознаванием прервано"); };
  try {
    const offer = await pc.createOffer(); await pc.setLocalDescription(offer);
    const res = await fetch(`${API}/api/realtime/connect`, { method: "POST", headers: { "Content-Type": "application/sdp" }, body: offer.sdp, signal: AbortSignal.timeout(25000) });
    if (!res.ok) { const e = await res.json(); throw new Error(e.error || "Realtime недоступен"); }
    await pc.setRemoteDescription({ type: "answer", sdp: await res.text() }); await ctx.resume();
    await new Promise<void>((resolve, reject) => { if (dc.readyState === "open") return resolve(); const t = setTimeout(() => reject(new Error("Realtime connection timeout")), 15000); dc.onopen = () => { clearTimeout(t); resolve(); }; });
    if (closed) throw new Error("Соединение закрыто");
    onState("Слушаю"); deadline = setTimeout(() => fail("Лимит записи 30 секунд"), 30000);
    const samples = new Float32Array(analyser.fftSize);
    timer = setInterval(() => {
      analyser.getFloatTimeDomainData(samples); let energy = 0; for (const x of samples) energy += x * x; const rms = Math.sqrt(energy / samples.length);
      if (rms > 0.018) { voicedFrames++; lastVoice = performance.now(); if (voicedFrames >= 3) speaking = true; }
      if (speaking && performance.now() - lastVoice > 360) commit();
    }, 30);
    return { commit, close };
  } catch (e) { close(); throw e; }
}
