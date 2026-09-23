/** Recorded microphone audio → existing ElevenLabs Scribe HTTP gateway. */
export const runtime = "nodejs";
const MAX_AUDIO_BYTES = 20 * 1024 * 1024;

export async function POST(request: Request) {
  const length = Number(request.headers.get("content-length") ?? 0);
  if (length > MAX_AUDIO_BYTES + 64_000) return Response.json({ error: "Запись слишком большая (максимум 20 МБ)" }, { status: 413 });
  let form: FormData;
  try { form = await request.formData(); }
  catch { return Response.json({ error: "Ожидалась аудиозапись в поле file" }, { status: 400 }); }
  const file = form.get("file");
  if (!(file instanceof Blob) || file.size === 0) return Response.json({ error: "Пустая запись" }, { status: 400 });
  if (file.size > MAX_AUDIO_BYTES) return Response.json({ error: "Запись слишком большая (максимум 20 МБ)" }, { status: 413 });
  if (!file.type.startsWith("audio/") && file.type !== "video/webm") return Response.json({ error: "Ожидался аудиофайл" }, { status: 415 });
  const extension = file.type.includes("mp4") ? "mp4" : file.type.includes("wav") ? "wav" : file.type.includes("ogg") ? "ogg" : file.type.includes("mpeg") ? "mp3" : "webm";
  const body = new FormData();
  body.append("file", file, `recording.${extension}`);
  const t0 = Date.now();
  try {
    const base = (process.env.VOICE_STT_URL ?? process.env.VOICE_TTS_URL ?? "http://127.0.0.1:8091").replace(/\/$/, "");
    const response = await fetch(`${base}/api/voice/stt`, {
      method: "POST", body, cache: "no-store",
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(30_000)]),
    });
    if (!response.ok) {
      await response.body?.cancel();
      return Response.json({ error: "ElevenLabs не смог распознать речь. Проверьте ключ и квоту voice-сервиса." }, { status: 502 });
    }
    const result = await response.json();
    if (typeof result.text !== "string") throw new Error("Invalid transcription");
    return Response.json({ text: result.text.trim(), language: result.language, ms: Date.now() - t0, provider: "elevenlabs" });
  } catch {
    return Response.json({ error: "Сервис распознавания недоступен. Проверьте запуск voice и VOICE_STT_URL." }, { status: 503 });
  }
}
