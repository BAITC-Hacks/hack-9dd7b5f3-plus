const STEPS = [
  { n: "01", t: "Слышит", d: "Потоковое распознавание на русском и казахском, переключение языка внутри фразы." },
  { n: "02", t: "Понимает", d: "LLM читает описания сорока сценариев и правила «не этот, если…» вместе с историей диалога и выбирает сценарий с уверенностью." },
  { n: "03", t: "Решает", d: "Уверен — запускает сценарий. Сомневается — задаёт один короткий вопрос. Не справляется — передаёт оператору с контекстом." },
  { n: "04", t: "Действует и отвечает", d: "Находит клиента по телефону, считает цену по правилам, перед необратимым действием читает данные вслух и ждёт «да»." },
];

export function How() {
  return (
    <section id="how" className="border-y border-border bg-paper-2/60">
      <div className="mx-auto w-full max-w-[1200px] px-5 py-20 sm:px-8 lg:py-28">
        <div className="mb-12 max-w-2xl">
          <div className="text-sm text-brand">Как работает</div>
          <h2 className="mt-2 text-3xl font-medium tracking-[-0.02em] text-ink sm:text-4xl">Четыре шага за полторы секунды.</h2>
        </div>
        <ol className="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {STEPS.map((s) => (
            <li key={s.n} className="rounded-2xl border border-border bg-white p-6">
              <div className="font-mono text-xs text-brand">{s.n}</div>
              <div className="mt-3 text-xl font-medium tracking-tight text-ink">{s.t}</div>
              <p className="mt-2 text-[15px] leading-relaxed text-body">{s.d}</p>
            </li>
          ))}
        </ol>
        <div className="mt-8 rounded-2xl border border-border bg-white p-6">
          <div className="flex items-baseline justify-between text-sm"><span className="text-body">Бюджет времени, ориентир кейса</span><span className="font-medium tabular-nums text-ink">≤ 1 500 мс</span></div>
          <div className="mt-3 flex h-3 overflow-hidden rounded-full">
            <span className="bg-s3" style={{ width: "20%" }} title="Распознавание" />
            <span className="bg-s6" style={{ width: "33%" }} title="Выбор сценария ≤ 500 мс" />
            <span className="bg-s4" style={{ width: "27%" }} title="Ответ" />
            <span className="bg-s7" style={{ width: "20%" }} title="Озвучка" />
          </div>
          <div className="mt-2 grid grid-cols-4 text-xs text-body"><span>Распознавание</span><span>Выбор сценария ≤ 500 мс</span><span>Ответ</span><span className="text-right">Озвучка</span></div>
        </div>
      </div>
    </section>
  );
}
