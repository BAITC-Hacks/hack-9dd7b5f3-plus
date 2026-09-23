/** Same-origin proxy. CORE_LLM_URL is private server configuration. */
export const runtime = "nodejs";

export async function POST(request: Request) {
  let body: string;
  try {
    body = await request.text();
    if (Buffer.byteLength(body) > 32_768) return Response.json({ error: "Слишком длинный запрос" }, { status: 413 });
    JSON.parse(body);
  } catch {
    return Response.json({ error: "Ожидался JSON" }, { status: 400 });
  }
  try {
    const base = (process.env.CORE_LLM_URL ?? "http://127.0.0.1:8090").replace(/\/$/, "");
    const response = await fetch(`${base}/api/route`, {
      method: "POST", headers: { "Content-Type": "application/json" }, body,
      signal: AbortSignal.timeout(29_000), cache: "no-store",
    });
    if (!response.ok) return Response.json({ error: response.status === 400 ? "Некорректный запрос маршрутизации" : "Ошибка core-llm. Проверьте ключ и модель в core-llm/.env." }, { status: response.status === 400 ? 400 : 502 });
    return Response.json(await response.json());
  } catch {
    return Response.json({ error: "core-llm не отвечает. Запустите python3 core-llm/server.py и проверьте CORE_LLM_URL." }, { status: 503 });
  }
}
