import { Container } from "./section";

const METRICS = [
  { label: "Сценарии", value: "40", desc: "плюс 3 системных интента: уточнение, вне компетенции, завершение" },
  { label: "Языковые режимы", value: "ru · kk · mixed", desc: "переключение языка внутри фразы, ответ на языке клиента" },
  { label: "Выбор сценария", value: "≤ 500 мс", desc: "ориентир кейса, не измерение: решение роутера на ход" },
  { label: "До первого звука", value: "≤ 1,5 с", desc: "ориентир кейса: от конца речи до начала ответа TTS" },
];

export function NumbersStrip() {
  return (
    <section className="py-8">
      <Container>
        <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
          {METRICS.map((m) => (
            <div key={m.label} className="rounded-[12px] border border-border bg-card p-5">
              <div className="metric-label">{m.label}</div>
              <div className="metric-value text-ink mt-3 text-[2rem]">{m.value}</div>
              <div className="text-body mt-2 text-sm leading-snug">{m.desc}</div>
            </div>
          ))}
        </div>
      </Container>
    </section>
  );
}
