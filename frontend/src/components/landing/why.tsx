import { GlassPanel } from "./visuals";

const CASES = [
  { title: "Смена темы посреди диалога", quote: "Хочу узнать, что с моим заявлением по каско… а каско у меня скоро заканчивается — его можно продлить в рассрочку?", route: "SC17 → SC27 + SC31", note: "Робот отвечает про заявление, откладывает продление в стек и возвращается к нему." },
  { title: "Запрос на стыке двух сценариев", quote: "Отказали в выплате, и ваш оператор ещё и нагрубил", route: "SC19 + SC35", note: "Несогласие с решением и жалоба на сервис — два разных сценария в одной фразе." },
  { title: "Два языка в одной фразе", quote: "Кеше аварияға түстім, но я не виноват, виновник у вас застрахован", route: "SC12", note: "Смысл «я пострадавший» приходит в русской части, ответ идёт на казахском." },
];

export function Why() {
  return (
    <section id="why" className="mx-auto w-full max-w-[1200px] px-5 py-20 sm:px-8 lg:py-28">
      <div className="grid items-center gap-10 lg:grid-cols-2">
        <div>
          <div className="text-sm text-brand">Почему LLM, а не классификатор</div>
          <h2 className="mt-2 text-3xl font-medium tracking-[-0.02em] text-ink sm:text-4xl">Классификатор учится на фразах. Робот должен понимать людей.</h2>
          <p className="mt-4 text-lg leading-relaxed text-body">Энкодерный классификатор ломается там, где начинается живая речь. LLM-слой читает описание каждого сценария и его границы, видит историю диалога и выбирает с объяснением. Новый сценарий — это строка в каталоге, а не переобучение.</p>
        </div>
        <div className="relative h-[360px] overflow-hidden rounded-3xl border border-border sm:h-[420px]">
          <GlassPanel imageSrc="/landing/plus.svg" className="absolute inset-0" />
        </div>
      </div>
      <ul className="mt-14 grid gap-5 md:grid-cols-3">
        {CASES.map((c) => (
          <li key={c.title} className="flex flex-col rounded-2xl border border-border bg-white p-6">
            <div className="text-sm font-medium text-ink">{c.title}</div>
            <blockquote className="mt-3 flex-1 text-[15px] leading-relaxed text-body">«{c.quote}»</blockquote>
            <div className="mt-4 font-mono text-xs text-brand">{c.route}</div>
            <div className="mt-1 text-xs text-body">{c.note}</div>
          </li>
        ))}
      </ul>
    </section>
  );
}
