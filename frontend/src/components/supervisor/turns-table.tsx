"use client";

import Link from "next/link";
import { cn } from "@/lib/cn";
import type { ScenarioNames } from "@/lib/catalog-cache";
import { fmtTime, shortId, truncate } from "@/lib/format";
import type { TurnRecord } from "@/lib/types";
import { ConfidenceBar, type Thresholds } from "@/components/ui/confidence-bar";
import { LangBadge, PathBadge, PolicyBadge, ScenarioTag } from "@/components/ui/pill";
import { EmptyState } from "@/components/ui/states";
import { Table, Td, Th } from "@/components/ui/table";
import { FIRST_AUDIO_TARGET_MS, ROUTE_TARGET_MS } from "@/components/trace/timings-waterfall";

function msCell(v: number | undefined, target: number) {
  if (v === undefined || v === 0) return <span className="text-muted-foreground/40">—</span>;
  return <span className={cn("tabular-nums", v > target ? "text-warning" : "text-success")}>{v}</span>;
}

export function TurnsTable({
  rows,
  names,
  thresholds,
  showSession = true,
  emptyHint,
}: {
  rows: TurnRecord[];
  names: ScenarioNames;
  thresholds: Thresholds;
  showSession?: boolean;
  emptyHint?: string;
}) {
  if (rows.length === 0) return <EmptyState title="No turns yet" hint={emptyHint ?? "Turns appear here as soon as someone talks to the robot."} />;
  return (
    <Table>
      <thead>
        <tr>
          <Th>time</Th>
          {showSession && <Th>session</Th>}
          <Th>transcript</Th>
          <Th>lang</Th>
          <Th>scenarios</Th>
          <Th>confidence</Th>
          <Th>path</Th>
          <Th>policy</Th>
          <Th className="text-right">route ms</Th>
          <Th className="text-right">first audio</Th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={`${r.session_id}-${r.turn}`} className="hover:bg-foreground/[0.025]">
            <Td className="whitespace-nowrap tabular-nums text-muted-foreground">{fmtTime(r.at)}</Td>
            {showSession && (
              <Td className="whitespace-nowrap">
                <Link href={`/supervisor/${encodeURIComponent(r.session_id)}`} className="font-mono text-[11.5px] tracking-normal text-primary hover:underline">
                  {shortId(r.session_id, 8)}
                </Link>
                <span className="ml-1 text-[10.5px] text-muted-foreground/50">#{r.turn}</span>
              </Td>
            )}
            <Td className="max-w-[340px]">
              <span className="block truncate" title={r.transcript}>
                {truncate(r.transcript, 90)}
              </span>
            </Td>
            <Td>
              <LangBadge lang={r.language} />
            </Td>
            <Td>
              <span className="flex flex-wrap gap-1">
                {(r.scenarios ?? []).length ? (r.scenarios ?? []).map((s) => <ScenarioTag key={s} id={s} name={names[s]} />) : <span className="text-muted-foreground/40">—</span>}
              </span>
            </Td>
            <Td>
              <ConfidenceBar value={r.confidence} thresholds={thresholds} width="w-12" />
            </Td>
            <Td>
              <PathBadge path={r.path} />
            </Td>
            <Td>
              <PolicyBadge action={r.policy_action} />
            </Td>
            <Td className="text-right">{msCell(r.timings?.route, ROUTE_TARGET_MS)}</Td>
            <Td className="text-right">{msCell(r.timings?.first_audio, FIRST_AUDIO_TARGET_MS)}</Td>
          </tr>
        ))}
      </tbody>
    </Table>
  );
}
