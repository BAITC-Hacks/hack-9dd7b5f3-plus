"use client";
/** «Журнал»: one row per finished turn. */
import { LangBadge } from "@/components/app/conversation-panel";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { scenarioLabel } from "@/lib/catalog";
import { CONFIDENCE_RUN, type Trace } from "@/lib/contract";
import { cn } from "@/lib/utils";
import { fmtMs, fmtPct, PolicyBadge, Section } from "./shared";

export function TurnLog({ traces }: { traces: Trace[] }) {
  return (
    <Section title="Журнал реплик" hint="жёлтым — уверенность ниже 75%" bodyClassName="px-0 pb-0">
      {traces.length === 0 ? (
        <div className="px-4 pb-4 text-sm text-muted-foreground">Пока пусто. Каждая реплика появится здесь со сценарием, уверенностью и временем ответа.</div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">#</TableHead>
              <TableHead>Реплика</TableHead>
              <TableHead>Язык</TableHead>
              <TableHead>Сценарий</TableHead>
              <TableHead className="text-right">Уверенность</TableHead>
              <TableHead>Что сделал</TableHead>
              <TableHead className="text-right">мс</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {[...traces].reverse().map((t) => {
              const top = t.scenarios[0];
              const low = (top?.confidence ?? 0) < CONFIDENCE_RUN;
              return (
                <TableRow key={t.turn}>
                  <TableCell className="font-mono text-xs text-muted-foreground">{t.turn}</TableCell>
                  <TableCell className="max-w-[320px] truncate" title={t.transcript}>{t.transcript}</TableCell>
                  <TableCell><LangBadge lang={t.language} /></TableCell>
                  <TableCell>
                    {top ? (
                      <div className="flex flex-col">
                        <span>{scenarioLabel(top.scenario_id, "ru")}{t.scenarios.length > 1 && <span className="text-muted-foreground"> +{t.scenarios.length - 1}</span>}</span>
                        <span className="font-mono text-xs text-muted-foreground">{top.scenario_id}</span>
                      </div>
                    ) : "—"}
                  </TableCell>
                  <TableCell className={cn("text-right font-mono text-xs tabular-nums", low && "text-warning-foreground")}>{fmtPct(top?.confidence)}</TableCell>
                  <TableCell><PolicyBadge action={t.policy?.action} size="sm" /></TableCell>
                  <TableCell className={cn("text-right font-mono text-xs tabular-nums", (t.latency_ms.total ?? 0) > 1500 && "text-warning-foreground")}>{fmtMs(t.latency_ms.total)}</TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      )}
    </Section>
  );
}
