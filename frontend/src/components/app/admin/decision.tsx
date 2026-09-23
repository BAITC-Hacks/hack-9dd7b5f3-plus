"use client";
import { useUiLanguage, translate as t } from "@/lib/ui-language";
/** «Что понял робот»: chosen scenario, confidence, why, live candidates, catalogue rules. */
import { Badge } from "@/components/ui/badge";
import { isSystemIntent, scenarioById, scenarioLabel, systemIntentById } from "@/lib/catalog";
import { cn } from "@/lib/utils";
import { Bar, Dash, fmtPct, Section, type TurnView } from "./shared";

export function DecisionCard({ view }: { view: TurnView }) {
  const language = useUiLanguage();
  const d = view.decision;
  const top = d?.scenarios[0];
  const sc = top ? scenarioById(top.scenario_id) : undefined;
  const sys = top && isSystemIntent(top.scenario_id) ? systemIntentById(top.scenario_id) : undefined;
  const routing = view.source === "live" && view.stage !== "done" && !d;
  const cands = [...view.candidates].sort((a, b) => b.confidence - a.confidence).slice(0, 5);
  const low = top && top.confidence < 0.75;

  return (
    <Section
      title={t("Что понял робот")}
      hint={view.transcript ? `«${view.transcript}»` : undefined}
      right={sc?.priority === "urgent" ? <Badge variant="error" size="sm">{t("срочно")}</Badge> : d?.is_continuation ? <Badge variant="info" size="sm">{t("продолжение")}</Badge> : undefined}
    >
      {!top && !routing && <div className="py-6 text-center text-sm text-muted-foreground">{t("Скажите или напишите что-нибудь слева.")}</div>}
      {routing && <div className="flex items-center gap-2 py-2 text-sm text-muted-foreground"><span className="size-1.5 animate-pulse rounded-full bg-brand" />{t("выбираю сценарий…")}</div>}

      {top && (
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="text-xs text-muted-foreground">{t("Сценарий")}</div>
            <div className="mt-0.5 text-lg font-medium leading-snug">{scenarioLabel(top.scenario_id, language)}</div>
            <div className="mt-0.5 font-mono text-xs text-muted-foreground">{top.scenario_id}{sc ? ` · ${sc.category}` : ""}</div>
          </div>
          <div className="shrink-0 text-right">
            <div className="text-xs text-muted-foreground">{t("Уверенность")}</div>
            <div className={cn("mt-0.5 text-2xl font-medium tabular-nums", low && "text-warning-foreground")}>{fmtPct(top.confidence)}</div>
          </div>
        </div>
      )}

      {top && (
        <div className="mt-3 rounded-lg bg-muted px-3 py-2 text-sm leading-relaxed">
          <span className="text-muted-foreground">{t("Почему:")}</span>{d?.reason || top.reason || <Dash />}
        </div>
      )}

      {d && d.scenarios.length > 1 && (
        <div className="mt-3 text-sm">
          <div className="text-xs text-muted-foreground">{t("В этой реплике несколько просьб, по порядку:")}</div>
          <ol className="mt-1 space-y-0.5">
            {d.scenarios.map((s, i) => (
              <li key={s.scenario_id} className="flex justify-between gap-3">
                <span>{i + 1}. {scenarioLabel(s.scenario_id, language)} <span className="font-mono text-xs text-muted-foreground">{s.scenario_id}</span></span>
                <span className="font-mono text-xs tabular-nums text-muted-foreground">{fmtPct(s.confidence)}</span>
              </li>
            ))}
          </ol>
        </div>
      )}

      {cands.length > 0 && (
        <div className="mt-4">
          <div className="mb-1.5 flex items-center justify-between text-xs text-muted-foreground">
            <span>{t("Кандидаты")}{view.source === "live" && view.stage === "router" ? t(" — обновляются") : ""}</span>
            <span>{t("уверенность")}</span>
          </div>
          <ul className="space-y-1.5">
            {cands.map((c) => {
              const chosen = top?.scenario_id === c.scenario_id;
              return (
                <li key={c.scenario_id} className="grid grid-cols-[minmax(0,1fr)_120px_44px] items-center gap-3 text-sm">
                  <span className={cn("truncate", !chosen && "text-muted-foreground")}>{scenarioLabel(c.scenario_id, language)} <span className="font-mono text-xs text-muted-foreground/70">{c.scenario_id}</span></span>
                  <Bar value={c.confidence} fill={chosen ? "bg-s6" : "bg-s4/70"} />
                  <span className="text-right font-mono text-xs tabular-nums text-muted-foreground">{fmtPct(c.confidence)}</span>
                </li>
              );
            })}
          </ul>
        </div>
      )}

      {sc && sc.not_this_if.length > 0 && (
        <details className="mt-4 text-sm">
          <summary className="cursor-pointer text-xs text-muted-foreground hover:text-foreground">{t("Правила разграничения из каталога (")}{sc.not_this_if.length})</summary>
          <ul className="mt-2 space-y-1.5">
            {sc.not_this_if.map((r, i) => (
              <li key={i} className="rounded-lg bg-muted px-3 py-2 text-xs leading-relaxed">
                <span className="text-muted-foreground">{t("Не этот сценарий, если:")}</span>{r.condition}
                <span className="text-muted-foreground"> → </span><span className="font-mono">{r.use_instead}</span> {scenarioLabel(r.use_instead, language)}
              </li>
            ))}
          </ul>
        </details>
      )}

      {sys && <div className="mt-3 text-xs text-muted-foreground">{sys.description}</div>}
    </Section>
  );
}
