import { Badge } from "@/components/ui/badge";
import { scenarioLabel } from "@/lib/catalog";
import { Container, SectionHead } from "./section";

const CASES = [
  {
    n: "01",
    title: "Смена темы посреди диалога",
    quote: "Здравствуйте, хочу узнать, что с моим заявлением по каско… а каско у меня скоро заканчивается — его можно продлить в рассрочку?",
    from: ["SC17"],
    to: ["SC27", "SC31"],
    note: "Классификатор видит один интент на фразу. Роутер держит стек тем: возвращает статус, затем переходит к продлению и рассрочке.",
  },
  {
    n: "02",
    title: "Запрос на стыке двух сценариев",
    quote: "Отказали в выплате, и ваш оператор ещё и нагрубил",
    from: [],
    to: ["SC19", "SC35"],
    note: "Две задачи в одной реплике: оспорить отказ и оформить жалобу. Роутер отдаёт основной сценарий и альтернативу с уверенностью.",
  },
  {
    n: "03",
    title: "Казахский внутри русской фразы",
    quote: "Кеше аварияға түстім, но я не виноват, виновник у вас застрахован",
    from: [],
    to: ["SC12"],
    note: "Code-switching ломает encoder на этапе токенов. LLM читает обе части, определяет язык ответа и роль пострадавшего.",
  },
];

export function WhyClassifier() {
  return (
    <section className="bg-paper-2 py-16 md:py-24">
      <Container>
        <SectionHead index="03" marker="Маршрутизация" title="Почему классификатор ломается" meta="3 случая · реальные фразы из кейса" />
        <div className="grid gap-4 md:grid-cols-3">
          {CASES.map((c) => (
            <article key={c.n} className="flex flex-col rounded-[12px] border border-border bg-card p-5">
              <span className="marker">{c.n}</span>
              <h3 className="font-heading text-ink mt-3 text-xl font-semibold tracking-[-0.022em]">{c.title}</h3>
              <blockquote className="text-ink mt-4 border-l-2 border-brand pl-4 text-[15px] leading-relaxed">«{c.quote}»</blockquote>
              <p className="text-body mt-4 text-sm leading-relaxed">{c.note}</p>
              <div className="mt-auto flex flex-wrap items-center gap-1.5 pt-5">
                {c.from.map((id) => (
                  <Badge key={id} variant="outline" className="font-mono" title={scenarioLabel(id)}>
                    {id}
                  </Badge>
                ))}
                {c.from.length > 0 ? <span className="marker px-1">→</span> : null}
                {c.to.map((id, i) => (
                  <span key={id} className="flex items-center gap-1.5">
                    {i > 0 ? <span className="marker">+</span> : null}
                    <Badge variant="info" className="font-mono" title={scenarioLabel(id)}>
                      {id}
                    </Badge>
                  </span>
                ))}
              </div>
              <div className="text-body mt-2 text-xs">{c.to.map((id) => scenarioLabel(id)).join(" · ")}</div>
            </article>
          ))}
        </div>
      </Container>
    </section>
  );
}
