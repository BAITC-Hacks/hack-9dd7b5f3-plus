import { Container, SectionHead } from "./section";

const STEPS: { n: string; name: string; caption: string }[] = [
  { n: "01", name: "Микрофон", caption: "Push-to-talk, VAD по паузе" },
  { n: "02", name: "STT", caption: "Потоковое распознавание ru/kk" },
  { n: "03", name: "Триаж", caption: "Срочность, язык, короткие ответы" },
  { n: "04", name: "LLM-роутер", caption: "Сценарий + уверенность + причина" },
  { n: "05", name: "Политика", caption: "Порог, стек тем, подтверждение" },
  { n: "06", name: "Исполнитель", caption: "Слоты, mock-бэкенд, действия" },
  { n: "07", name: "Ответ", caption: "Короткая реплика на языке клиента" },
  { n: "08", name: "TTS", caption: "Первый звук ≤ 1,5 с" },
];

// Latency budget (targets from the case, not measurements).
const BUDGET: { name: string; ms: number; tone: string }[] = [
  { name: "STT", ms: 300, tone: "bg-s3" },
  { name: "Роутер", ms: 500, tone: "bg-s6" },
  { name: "Ответ", ms: 400, tone: "bg-s4" },
  { name: "TTS", ms: 300, tone: "bg-s2" },
];
const TOTAL = BUDGET.reduce((a, b) => a + b.ms, 0);

export function Pipeline() {
  return (
    <section className="py-16 md:py-24">
      <Container>
        <SectionHead index="02" marker="Конвейер" title="Как это работает" meta="8 стадий · one-shot LLM · стриминг" />

        <ol className="grid grid-cols-2 gap-2 md:grid-cols-4 lg:grid-cols-8">
          {STEPS.map((s, i) => (
            <li key={s.n} className="relative flex flex-col gap-3 rounded-[12px] border border-border bg-card p-4">
              <span className={"marker" + (s.n === "04" ? " marker-dot" : "")}>{s.n}</span>
              <div>
                <div className="font-heading text-ink text-[15px] font-semibold tracking-[-0.01em]">{s.name}</div>
                <div className="text-body mt-1 text-xs leading-snug">{s.caption}</div>
              </div>
              {i < STEPS.length - 1 ? (
                <span aria-hidden className="absolute top-1/2 -right-[5px] hidden size-2 rotate-45 border-t border-r border-border bg-card lg:block" />
              ) : null}
            </li>
          ))}
        </ol>

        <div className="mt-8 rounded-[12px] border border-border bg-card p-4 md:p-5">
          <div className="flex items-baseline justify-between gap-4">
            <span className="marker">Бюджет задержки · ориентир</span>
            <span className="num text-ink text-sm">{TOTAL.toLocaleString("ru-RU")} мс</span>
          </div>
          <div className="mt-3 flex h-3 w-full overflow-hidden rounded-[4px] bg-s0">
            {BUDGET.map((b) => (
              <span key={b.name} className={b.tone} style={{ width: `${(b.ms / TOTAL) * 100}%` }} title={`${b.name} ~${b.ms} мс`} />
            ))}
          </div>
          <div className="mt-3 grid grid-cols-2 gap-x-6 gap-y-2 sm:grid-cols-4">
            {BUDGET.map((b) => (
              <div key={b.name} className="flex items-center justify-between gap-3 text-sm">
                <span className="flex items-center gap-2 text-body">
                  <span className={"inline-block size-2 " + b.tone} /> {b.name}
                </span>
                <span className="num text-ink">{b.name === "Роутер" ? "≤ " : "~"}{b.ms}</span>
              </div>
            ))}
          </div>
          <p className="marker mt-4">Цели кейса: выбор сценария ≤ 500 мс · конец речи → первый звук ≤ 1,5 с</p>
        </div>
      </Container>
    </section>
  );
}
