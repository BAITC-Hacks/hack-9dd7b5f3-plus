import { scenarioNames, scenarios } from "@/lib/catalog";

const CAT: Record<string, { label: string; cls: string }> = {
  sales: { label: "продажи", cls: "bg-s4" },
  claims: { label: "страховые случаи", cls: "bg-s7" },
  servicing: { label: "обслуживание", cls: "bg-s5" },
  info: { label: "справка", cls: "bg-s2" },
  feedback: { label: "жалобы", cls: "bg-s6" },
  contact: { label: "связь", cls: "bg-s3" },
  security: { label: "безопасность", cls: "bg-s7" },
};

export function Scenarios() {
  const urgent = scenarios.filter((s) => s.priority === "urgent").length;
  const confirm = scenarios.filter((s) => s.requires_confirmation).length;
  const ident = scenarios.filter((s) => s.requires_identification).length;
  return (
    <section className="relative z-10 mx-auto grid w-full max-w-[1200px] items-start gap-12 px-5 py-20 sm:px-8 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] lg:py-28">
      <div>
        <h2 className="text-3xl font-medium tracking-[-0.02em] text-ink sm:text-[40px] sm:leading-[1.1]">Сорок сценариев. Один каталог, ноль переобучения.</h2>
        <p className="mt-5 text-[17px] leading-relaxed text-body">Роутер читает описание каждого сценария и его границы «не этот, если…». Новый сценарий — это строка в каталоге, а не новая модель.</p>
        <dl className="mt-8 grid grid-cols-3 gap-4 border-t border-border pt-6">
          <div><dt className="text-sm text-body">срочных</dt><dd className="mt-1 text-3xl font-medium tabular-nums text-ink">{urgent}</dd></div>
          <div><dt className="text-sm text-body">с подтверждением</dt><dd className="mt-1 text-3xl font-medium tabular-nums text-ink">{confirm}</dd></div>
          <div><dt className="text-sm text-body">с идентификацией</dt><dd className="mt-1 text-3xl font-medium tabular-nums text-ink">{ident}</dd></div>
        </dl>
      </div>
      <div className="rounded-3xl border border-border bg-white p-5 sm:p-6">
        <div className="grid grid-cols-8 gap-1.5 sm:gap-2">
          {scenarios.map((s) => {
            const c = CAT[s.category] ?? CAT.info;
            return (
              <div key={s.scenario_id} title={`${s.scenario_id} · ${scenarioNames.ru[s.scenario_id]}`} className={`group relative aspect-square rounded-md ${c.cls} transition-transform hover:scale-[1.06]`}>
                <span className="absolute inset-0 grid place-items-center font-mono text-[10px] text-white/90 sm:text-xs">{s.scenario_id.slice(2)}</span>
                {s.priority === "urgent" && <span className="absolute right-1 top-1 size-1.5 rounded-full bg-white" />}
              </div>
            );
          })}
        </div>
        <div className="mt-5 flex flex-wrap gap-x-5 gap-y-2 text-xs text-body">
          {Object.entries(CAT).filter(([k]) => k !== "security").map(([k, v]) => (
            <span key={k} className="inline-flex items-center gap-1.5"><span className={`size-2.5 rounded-sm ${v.cls}`} />{v.label}</span>
          ))}
          <span className="inline-flex items-center gap-1.5"><span className="size-1.5 rounded-full bg-ink" />срочный</span>
        </div>
      </div>
    </section>
  );
}
