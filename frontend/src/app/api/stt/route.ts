/**
 * POST /api/stt — server-side speech recognition (fallback when the browser's Web Speech API
 * cannot reach Google: Arc/Brave/Yandex browsers, restricted venue networks).
 * Body: multipart/form-data { file: audio/webm, language?: "ru" | "kk" }. Uses OpenAI transcription;
 * key comes from frontend/.env.local (OPENAI_API_KEY), never from the client.
 */
import { NextResponse } from "next/server";

export const runtime = "nodejs";

export async function POST(req: Request) {
  const key = process.env.OPENAI_API_KEY;
  if (!key) return NextResponse.json({ error: "Серверное распознавание не настроено: добавьте OPENAI_API_KEY в frontend/.env.local и перезапустите npm run dev." }, { status: 503 });
  let form: FormData;
  try {
    form = await req.formData();
  } catch {
    return NextResponse.json({ error: "Ожидался multipart/form-data с полем file" }, { status: 400 });
  }
  const file = form.get("file");
  const language = String(form.get("language") ?? "");
  if (!(file instanceof Blob) || file.size === 0) return NextResponse.json({ error: "Пустая запись" }, { status: 400 });

  const fd = new FormData();
  fd.append("file", file, "audio.webm");
  fd.append("model", process.env.OPENAI_STT_MODEL ?? "gpt-4o-mini-transcribe");
  fd.append("response_format", "json");
  if (language === "ru" || language === "kk") fd.append("language", language);
  fd.append("prompt", "Звонок в страховую компанию Saqta Insurance. ОГПО, КАСКО, ДМС, полис, заявление, ИИН. Речь может быть на русском и казахском в одной фразе.");

  const t0 = Date.now();
  const r = await fetch("https://api.openai.com/v1/audio/transcriptions", { method: "POST", headers: { Authorization: `Bearer ${key}` }, body: fd });
  if (!r.ok) {
    const detail = await r.text();
    return NextResponse.json({ error: `OpenAI STT: HTTP ${r.status} ${detail.slice(0, 200)}` }, { status: 502 });
  }
  const j = (await r.json()) as { text?: string };
  return NextResponse.json({ text: (j.text ?? "").trim(), ms: Date.now() - t0 });
}
