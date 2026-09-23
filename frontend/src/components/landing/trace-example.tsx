import { Badge } from "@/components/ui/badge";
import { Container, SectionHead } from "./section";

const LATENCY = [
  { stage: "stt", ms: 280 },
  { stage: "router", ms: 410 },
  { stage: "response", ms: 350 },
  { stage: "tts", ms: 220 },
];
const TOTAL = LATENCY.reduce((a, b) => a + b.ms, 0);

const ALTS = [
  { id: "SC11", name: "Оформление ДТП, виновник", p: 0.2 },
  { id: "SC13", name: "Статус выплаты по ОГПО", p: 0.14 },
];

export function TraceExample() {
  return (
    <section className="py-16 md:py-24">
      <Container>
        <SectionHead index="04" marker="Супервизор" title="Что видит супервизор" meta="каждый ход · сценарий · причина · альтернативы · задержки" />
        <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
          <div className="flex flex-col gap-6">
            <p className="text-body max-w-md text-base leading-relaxed">
              Роутер возвращает не метку, а объяснение: выбранный сценарий, уверенность, причину выбора и альтернативы.
              Перед необратимыми действиями робот спрашивает подтверждение, а при переводе на оператора передаёт контекст целиком.
            </p>
            <ul className="text-body flex flex-col text-sm">
              {[
                "Транскрипт и язык каждой реплики",
                "Сценарий и живая уверенность по кандидатам",
                "Причина и альтернативы с оценками",
                "Задержка по стадиям и итог",
                "Действия: подтверждение, handoff, эскалация",
              ].map((t) => (
                <li key={t} className="hairline-b flex items-baseline gap-3 py-2.5">
                  <span aria-hidden className="inline-block size-1.5 shrink-0 translate-y-[-2px] bg-brand" />
                  {t}
                </li>
              ))}
            </ul>
          </div>

          <div className="rounded-[12px] border border-border bg-card p-5">
            <div className="flex items-center justify-between gap-4">
              <span className="marker marker-dot">Пример трассировки</span>
              <Badge variant="outline" className="font-mono">
                mock · turn 3
              </Badge>
            </div>

            <div className="mt-5 grid gap-5 md:grid-cols-2">
              <div className="flex flex-col gap-4">
                <div>
                  <div className="metric-label">Транскрипт · kk/ru mixed</div>
                  <p className="text-ink mt-1.5 text-[15px] leading-relaxed">«Кеше аварияға түстім, но я не виноват, виновник у вас застрахован»</p>
                </div>
                <div>
                  <div className="metric-label">Сценарий</div>
                  <p className="text-ink mt-1.5 text-[15px] font-medium">
                    <span className="font-mono">SC12</span> · Обращение пострадавшего по ОГПО виновника
                  </p>
                  <div className="mt-2 flex items-center gap-3">
                    <div className="h-1.5 flex-1 overflow-hidden rounded-[4px] bg-s0">
                      <div className="h-full bg-s6" style={{ width: "88%" }} />
                    </div>
                    <span className="num text-ink text-sm">0.88</span>
                  </div>
                </div>
                <div>
                  <div className="metric-label">Причина</div>
                  <p className="text-body mt-1.5 text-sm leading-relaxed">
                    Клиент — пострадавший, а не виновник: «я не виноват» и «виновник у вас застрахован». Нет вопросов о статусе
                    выплаты, поэтому не SC13.
                  </p>
                </div>
                <div>
                  <div className="metric-label">Альтернативы</div>
                  <ul className="mt-1.5 flex flex-col">
                    {ALTS.map((a) => (
                      <li key={a.id} className="hairline-b flex items-center gap-3 py-2 text-sm last:border-b-0">
                        <span className="font-mono text-ink">{a.id}</span>
                        <span className="text-body flex-1 truncate">{a.name}</span>
                        <span className="h-1 w-16 overflow-hidden rounded-[4px] bg-s0">
                          <span className="block h-full bg-s4" style={{ width: `${a.p * 100}%` }} />
                        </span>
                        <span className="num text-ink w-10">{a.p.toFixed(2)}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>

              <div className="flex flex-col gap-4">
                <div>
                  <div className="metric-label">Задержка · мс</div>
                  <table className="mt-1.5 w-full text-sm">
                    <tbody>
                      {LATENCY.map((l, i) => (
                        <tr key={l.stage} className="hairline-b">
                          <td className="num text-body w-6 py-2.5 pr-3 text-xs">{i + 1}</td>
                          <td className="py-2.5 font-mono text-ink">{l.stage}</td>
                          <td className="py-2.5">
                            <span className="block h-1 overflow-hidden rounded-[4px] bg-s0">
                              <span className="block h-full bg-s5" style={{ width: `${(l.ms / TOTAL) * 100}%` }} />
                            </span>
                          </td>
                          <td className="num text-ink py-2.5 pl-3">{l.ms}</td>
                        </tr>
                      ))}
                      <tr>
                        <td />
                        <td className="py-2.5 font-mono font-medium text-ink">total</td>
                        <td />
                        <td className="num text-ink py-2.5 pl-3 font-medium">{TOTAL.toLocaleString("ru-RU")}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
                <div>
                  <div className="metric-label">Вердикт</div>
                  <div className="mt-1.5 flex flex-wrap gap-1.5">
                    <Badge variant="success">Routable</Badge>
                    <Badge variant="outline" className="font-mono">
                      lang: kk
                    </Badge>
                    <Badge variant="outline" className="font-mono">
                      urgent: false
                    </Badge>
                  </div>
                </div>
                <div>
                  <div className="metric-label">Ответ робота</div>
                  <p className="text-ink mt-1.5 text-sm leading-relaxed">
                    Түсіндім. Кінәлі жақтың полисі бізде — өтінімді қабылдаймын. Оқиға қашан және қайда болды?
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Container>
    </section>
  );
}
