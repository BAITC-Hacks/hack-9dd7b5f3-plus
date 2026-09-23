const STEPS = [
  { t: "Слышит", d: "Потоковое распознавание, русский и казахский в одной фразе." },
  { t: "Понимает", d: "LLM выбирает сценарий по описаниям и правилам границ, с уверенностью и альтернативами." },
  { t: "Решает", d: "Уверен — запускает. Сомневается — переспрашивает. Не справляется — передаёт оператору с контекстом." },
  { t: "Действует", d: "Находит клиента по телефону, считает цену, читает данные вслух и ждёт «да» перед необратимым." },
];

const TRACE = `{
  "turn": 3,
  "transcript": "Кеше аварияға түстім, но я не виноват…",
  "language": "mixed",
  "scenarios": [{ "scenario_id": "SC12", "confidence": 0.90 }],
  "alternatives": [{ "scenario_id": "SC13", "confidence": 0.31 }],
  "reason": "клиент не виноват, виновник застрахован у нас",
  "slots": { "incident_date": "2026-09-30" },
  "actions": ["get_policy", "create_claim:preview"],
  "latency_ms": { "stt": 210, "router": 390, "response": 240,
                  "tts_first_audio": 120, "total": 960 }
}`;

function Code({ src }: { src: string }) {
  const parts = src.split(/("[^"]*")/g);
  return (
    <pre className="overflow-x-auto font-mono text-[13px] leading-[1.7] text-ink">
      {parts.map((p, i) => {
        if (!p.startsWith('"')) return <span key={i} className="text-body">{p}</span>;
        const isKey = src.indexOf(p + ":") >= 0 && src.indexOf(p + ":") === src.indexOf(p);
        return <span key={i} className={isKey ? "text-ink" : "text-brand"}>{p}</span>;
      })}
    </pre>
  );
}

export function TraceSection() {
  return (
    <section className="relative z-10 mx-auto grid w-full max-w-[1200px] items-start gap-12 px-5 py-20 sm:px-8 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] lg:py-28">
      <div>
        <h2 className="text-3xl font-medium tracking-[-0.02em] text-ink sm:text-[40px] sm:leading-[1.1]">Каждая реплика оставляет след.</h2>
        <p className="mt-5 text-[17px] leading-relaxed text-body">Четыре шага, и все они видны супервизору в формате из стартового кита.</p>
        <ol className="mt-8 divide-y divide-border border-y border-border">
          {STEPS.map((s, i) => (
            <li key={s.t} className="grid grid-cols-[40px_1fr] gap-3 py-4">
              <span className="font-mono text-sm text-brand">0{i + 1}</span>
              <div><div className="text-[17px] font-medium text-ink">{s.t}</div><div className="mt-0.5 text-[15px] leading-relaxed text-body">{s.d}</div></div>
            </li>
          ))}
        </ol>
      </div>
      <div className="lift rounded-3xl border border-border bg-white">
        <div className="flex items-center justify-between border-b border-border px-5 py-3 text-xs text-body">
          <span className="flex items-center gap-1.5"><span className="size-2.5 rounded-full bg-paper-2 ring-1 ring-border" /><span className="size-2.5 rounded-full bg-paper-2 ring-1 ring-border" /><span className="size-2.5 rounded-full bg-paper-2 ring-1 ring-border" /></span>
          <span className="font-mono">trace · turn 3</span>
        </div>
        <div className="p-5 sm:p-6"><Code src={TRACE} /></div>
      </div>
    </section>
  );
}
