"use client";
/** Журнал ходов — data table of finished traces. */
import { Mic } from "lucide-react";
import Link from "next/link";
import { LangBadge } from "@/components/app/conversation-panel";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { scenarioLabel } from "@/lib/catalog";
import { CONFIDENCE_RUN, type Trace } from "@/lib/contract";
import { cn } from "@/lib/utils";
import { fmtMs, fmtPct, PolicyBadge } from "./shared";

export function TurnLog({ traces }: { traces: Trace[] }) {
  return (
    <Card className="rounded-xl shadow-none before:hidden">
      <div className="hairline-b flex items-center justify-between gap-3 px-4 py-2.5">
        <div className="marker marker-dot">Журнал ходов</div>
        <span className="marker">{traces.length} строк · подсветка — уверенность &lt; {Math.round(CONFIDENCE_RUN * 100)} %</span>
      </div>
      {traces.length === 0 ? (
        <Empty className="py-10 md:py-14">
          <EmptyHeader>
            <EmptyMedia variant="icon"><Mic /></EmptyMedia>
            <EmptyTitle>Пока нет ходов</EmptyTitle>
            <EmptyDescription>Каждая реплика клиента появится здесь с трассировкой: сценарий, уверенность, вердикт политики и задержка по этапам.</EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button variant="default" size="sm" render={<Link href="/call" />}>Открыть симулятор</Button>
            <div className="text-xs text-muted-foreground">Говорить можно и здесь — микрофон в панели слева.</div>
          </EmptyContent>
        </Empty>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10 font-mono text-[11px] uppercase tracking-[.1em]">#</TableHead>
              <TableHead className="font-mono text-[11px] uppercase tracking-[.1em]">транскрипт</TableHead>
              <TableHead className="font-mono text-[11px] uppercase tracking-[.1em]">язык</TableHead>
              <TableHead className="font-mono text-[11px] uppercase tracking-[.1em]">сценарий</TableHead>
              <TableHead className="text-right font-mono text-[11px] uppercase tracking-[.1em]">уверенность</TableHead>
              <TableHead className="font-mono text-[11px] uppercase tracking-[.1em]">политика</TableHead>
              <TableHead className="text-right font-mono text-[11px] uppercase tracking-[.1em]">действия</TableHead>
              <TableHead className="text-right font-mono text-[11px] uppercase tracking-[.1em]">total ms</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {[...traces].reverse().map((t) => {
              const top = t.scenarios[0];
              const low = (top?.confidence ?? 0) < CONFIDENCE_RUN;
              return (
                <TableRow key={t.turn} className={cn(low && "bg-row-hover")}>
                  <TableCell className="font-mono text-xs text-muted-foreground">{t.turn}</TableCell>
                  <TableCell className="max-w-[280px] truncate" title={t.transcript}>{t.transcript}</TableCell>
                  <TableCell><LangBadge lang={t.language} /></TableCell>
                  <TableCell>
                    {top ? (
                      <div className="flex flex-col gap-0.5">
                        <span className="font-mono text-xs">{top.scenario_id}{t.scenarios.length > 1 && <span className="text-muted-foreground"> +{t.scenarios.length - 1}</span>}</span>
                        <span className="text-xs text-muted-foreground">{scenarioLabel(top.scenario_id, "ru")}</span>
                      </div>
                    ) : "—"}
                  </TableCell>
                  <TableCell className={cn("num", low && "text-warning-foreground")}>{fmtPct(top?.confidence)}</TableCell>
                  <TableCell><PolicyBadge action={t.policy?.action} size="sm" /></TableCell>
                  <TableCell className="num text-muted-foreground">{t.actions.length}</TableCell>
                  <TableCell className={cn("num", (t.latency_ms.total ?? 0) > 1500 && "text-warning-foreground")}>{fmtMs(t.latency_ms.total)}</TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      )}
    </Card>
  );
}
