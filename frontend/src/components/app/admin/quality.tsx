"use client";
/** «Качество маршрутизации»: the official evaluate.py metrics, run on the dev set. */
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { scenarioLabel } from "@/lib/catalog";
import { devUtterances, runEval, type EvalResult } from "@/lib/eval";
import type { ApiMode } from "@/lib/api";
import { cn } from "@/lib/utils";
import { fmtPct, Section } from "./shared";

const GROUP_LABEL: Record<string, string> = {
  all: "Все реплики", "lang=ru": "Русский", "lang=kk": "Казахский", "lang=mixed": "Смешанные",
  "type=single": "Одна просьба", "type=multi_intent": "Несколько просьб", "type=out_of_scope": "Не наша тема", "type=unclear": "Непонятные",
};

export function QualityCard({ mode }: { mode: ApiMode }) {
  const [res, setRes] = useState<EvalResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [showErrors, setShowErrors] = useState(false);

  const run = async () => {
    setBusy(true); setErr(null);
    try { setRes(await runEval(mode)); } catch (e) { setErr(String(e)); } finally { setBusy(false); }
  };

  const all = res?.groups.find((g) => g.key === "all");

  return (
    <Section
      title="Качество маршрутизации"
      hint={`метрики из data/evaluate.py на dev-наборе (${devUtterances.length} реплик)`}
      right={<Button size="sm" onClick={run} disabled={busy}>{busy ? "Считаю…" : res ? "Прогнать ещё раз" : "Прогнать dev-набор"}</Button>}
      bodyClassName="px-0 pb-0"
    >
      {err && <div className="px-4 pb-4 text-sm text-destructive-foreground">{err}</div>}
      {!res && !err && (
        <div className="px-4 pb-4 text-sm text-muted-foreground">
          Три показателя, по которым жюри и мы меряем роутер: <span className="text-foreground">primary accuracy</span> — угадан главный сценарий; <span className="text-foreground">full match</span> — угаданы все сценарии реплики; <span className="text-foreground">intent recall</span> — доля найденных сценариев в репликах с несколькими просьбами.
        </div>
      )}
      {res && all && (
        <>
          <div className="grid grid-cols-3 gap-3 px-4 pb-3">
            <Big label="Primary accuracy" value={fmtPct(all.primary)} hint="главный сценарий верный" />
            <Big label="Full match" value={fmtPct(all.full)} hint="все сценарии реплики верные" />
            <Big label="Intent recall" value={res.intent_recall == null ? "—" : fmtPct(res.intent_recall)} hint="для реплик с несколькими просьбами" />
          </div>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Группа</TableHead>
                <TableHead className="text-right">n</TableHead>
                <TableHead className="text-right">primary</TableHead>
                <TableHead className="text-right">full</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {res.groups.map((g) => (
                <TableRow key={g.key} className={cn(g.key === "all" && "font-medium")}>
                  <TableCell>{GROUP_LABEL[g.key] ?? g.key}</TableCell>
                  <TableCell className="text-right font-mono text-xs tabular-nums text-muted-foreground">{g.n}</TableCell>
                  <TableCell className={cn("text-right font-mono text-xs tabular-nums", g.primary < 0.9 && "text-warning-foreground")}>{fmtPct(g.primary)}</TableCell>
                  <TableCell className={cn("text-right font-mono text-xs tabular-nums", g.full < 0.9 && "text-warning-foreground")}>{fmtPct(g.full)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <div className="flex items-center justify-between border-t border-border px-4 py-2.5 text-xs text-muted-foreground">
            <span>Роутер: {res.model} · {res.mean_ms.toFixed(2)} мс на реплику · ошибок {res.errors.length}</span>
            {res.errors.length > 0 && <button type="button" className="hover:text-foreground" onClick={() => setShowErrors((v) => !v)}>{showErrors ? "Скрыть ошибки" : "Показать ошибки"}</button>}
          </div>
          {showErrors && res.errors.length > 0 && (
            <ul className="divide-y divide-border border-t border-border">
              {res.errors.map((e) => (
                <li key={e.id} className="px-4 py-2 text-sm">
                  <div className="text-foreground">{e.text}</div>
                  <div className="mt-0.5 text-xs text-muted-foreground">
                    <span className="font-mono">{e.id}</span> · ожидали {e.expected.map((x) => `${x} ${scenarioLabel(x, "ru")}`).join(", ")} · получили <span className="text-warning-foreground">{e.got.length ? e.got.map((x) => `${x} ${scenarioLabel(x, "ru")}`).join(", ") : "ничего"}</span>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </Section>
  );
}

function Big({ label, value, hint }: { label: string; value: string; hint: string }) {
  return (
    <div className="rounded-lg bg-muted px-3 py-2.5">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-0.5 text-2xl font-medium tabular-nums">{value}</div>
      <div className="text-xs text-muted-foreground">{hint}</div>
    </div>
  );
}
