/** TTS-only bridge to the voice service from feat/voice-elevenlabs. */
export const runtime = "nodejs";

export async function POST(request: Request) {
  let input: { text: string; lang: "ru" | "kk" | "en" };
  try {
    const raw = await request.text();
    if (Buffer.byteLength(raw) > 24_000) return Response.json({ error: "Слишком длинный ответ для озвучки" }, { status: 413 });
    input = JSON.parse(raw);
    if (!input || typeof input.text !== "string" || !input.text.trim() || input.text.length > 6000 || !["ru", "kk", "en"].includes(input.lang)) {
      return Response.json({ error: "Ожидались text и lang: ru, kk или en" }, { status: 400 });
    }
  } catch {
    return Response.json({ error: "Некорректный запрос TTS" }, { status: 400 });
  }
  try {
    const base = (process.env.VOICE_TTS_URL ?? "http://127.0.0.1:8091").replace(/\/$/, "");
    const response = await fetch(`${base}/api/voice/tts`, {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text: input.text.trim(), lang: input.lang, format: "mp3_22050_32" }),
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(30_000)]), cache: "no-store",
    });
    if (!response.ok || !response.body || !response.headers.get("content-type")?.startsWith("audio/")) {
      await response.body?.cancel();
      return Response.json({ error: "ElevenLabs не смог озвучить ответ. Проверьте ключ, голос и доступную квоту voice-сервиса." }, { status: 502 });
    }
    return new Response(response.body, { headers: {
      "Content-Type": "audio/mpeg", "Cache-Control": "no-store",
      "X-TTS-Provider": "elevenlabs", "X-TTS-Model": response.headers.get("x-tts-model") ?? "",
    } });
  } catch {
    return Response.json({ error: "Сервис озвучки недоступен. Проверьте VOICE_TTS_URL и запуск voice." }, { status: 503 });
  }
}
