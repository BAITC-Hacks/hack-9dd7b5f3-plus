"use client";
import { useState } from "react";
import { scenarioNames, scenarios } from "@/lib/catalog";
import { cn } from "@/lib/utils";
import { CARD, SECTION, SectionHead } from "./section-head";

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
  const [hover, setHover] = useState<string | null>(null);
  const [cat, setCat] = useState<string | null>(null);
  const urgent = scenarios.filter((s) => s.priority === "urgent").length;
  const confirm = scenarios.filter((s) => s.requires_confirmation).length;
  const ident = scenarios.filter((s) => s.requires_identification).length;
  const active = hover ? scenarios.find((s) => s.scenario_id === hover) : null;
  return (
    <section className={SECTION}>
      <SectionHead title="Сорок сценариев. Один каталог, ноль переобучения." text="Роутер читает описание каждого сценария и его границы «не этот, если…». Новый сценарий — это строка в каталоге, а не новая модель." />
      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,4fr)_minmax(0,8fr)]">
      <div className={CARD + " p-6"}>
        <div className="text-sm text-body">Каталог из стартового кита</div>
        <dl className="mt-4 grid grid-cols-3 gap-4 border-t border-border pt-5 lg:grid-cols-1 lg:gap-5">
          <div><dt className="text-sm text-body">срочных</dt><dd className="mt-1 text-3xl font-medium tabular-nums text-ink">{urgent}</dd></div>
          <div><dt className="text-sm text-body">с подтверждением</dt><dd className="mt-1 text-3xl font-medium tabular-nums text-ink">{confirm}</dd></div>
          <div><dt className="text-sm text-body">с идентификацией</dt><dd className="mt-1 text-3xl font-medium tabular-nums text-ink">{ident}</dd></div>
        </dl>
      </div>
      <div className={CARD + " p-5 sm:p-6"} onMouseLeave={() => setHover(null)}>
        <div className="mb-4 flex h-6 items-center justify-between text-sm">
          <span className="text-ink">{active ? <><span className="font-mono text-xs text-brand">{active.scenario_id}</span> · {scenarioNames.ru[active.scenario_id]}</> : <span className="text-body">Наведите на сценарий</span>}</span>
          {active && <span className="text-xs text-body">{CAT[active.category]?.label}{active.priority === "urgent" ? " · срочный" : ""}</span>}
        </div>
        <div className="grid grid-cols-8 gap-1.5 sm:gap-2">
          {scenarios.map((s) => {
            const c = CAT[s.category] ?? CAT.info;
            const dim = cat && s.category !== cat;
            return (
              <button
                type="button"
                key={s.scenario_id}
                onMouseEnter={() => setHover(s.scenario_id)}
                onFocus={() => setHover(s.scenario_id)}
                aria-label={`${s.scenario_id} ${scenarioNames.ru[s.scenario_id]}`}
                className={cn("relative aspect-square rounded-md transition-all duration-200 outline-none focus-visible:ring-2 focus-visible:ring-brand", c.cls, hover === s.scenario_id ? "scale-[1.12] shadow-[0_8px_20px_-8px_rgba(47,106,209,0.6)]" : "hover:scale-[1.06]", dim && "opacity-25")}
              >
                <span className="absolute inset-0 grid place-items-center font-mono text-[10px] text-white/90 sm:text-xs">{s.scenario_id.slice(2)}</span>
                {s.priority === "urgent" && <span className="absolute right-1 top-1 size-1.5 rounded-full bg-white" />}
              </button>
            );
          })}
        </div>
        <div className="mt-5 flex flex-wrap gap-x-4 gap-y-2 text-xs text-body">
          {Object.entries(CAT).filter(([k]) => k !== "security").map(([k, v]) => (
            <button type="button" key={k} onMouseEnter={() => setCat(k)} onMouseLeave={() => setCat(null)} className={cn("inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 transition-colors hover:bg-s0 hover:text-ink", cat === k && "text-ink")}><span className={`size-2.5 rounded-sm ${v.cls}`} />{v.label}</button>
          ))}
          <span className="inline-flex items-center gap-1.5 px-2"><span className="size-1.5 rounded-full bg-ink" />срочный</span>
        </div>
      </div>
      </div>
    </section>
  );
}
