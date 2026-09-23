"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowDownToLine, Eraser, Pause, Play, Plug } from "lucide-react";
import { cn } from "@/lib/cn";
import { useDebugEvents } from "@/lib/use-sse";
import { EventRow } from "@/components/debug/event-row";
import { PageBody, PageHeader } from "@/components/shell/page-header";
import { Button } from "@/components/ui/button";
import { Pill } from "@/components/ui/pill";
import { EmptyState } from "@/components/ui/states";

export default function DebugPage() {
  const [paused, setPaused] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const [filter, setFilter] = useState("");
  const [sessionInput, setSessionInput] = useState("");
  const [session, setSession] = useState("");
  const [expanded, setExpanded] = useState<Record<number, boolean>>({});
  const { rows, status, dropped, clear, supported } = useDebugEvents({ session, replay: true, paused });
  const listRef = useRef<HTMLDivElement>(null);

  const filtered = useMemo(() => {
    const needle = filter.trim().toLowerCase();
    if (!needle) return rows;
    return rows.filter((r) => r.ev.type.toLowerCase().includes(needle) || (r.ev.session_id ?? "").toLowerCase().includes(needle));
  }, [rows, filter]);

  const scrollKey = `${filtered.length}:${filtered[filtered.length - 1]?.ev.t ?? 0}:${filtered[filtered.length - 1]?.merged ?? 0}`;
  useEffect(() => {
    if (!autoScroll) return;
    const el = listRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [scrollKey, autoScroll]);

  const dot = status === "open" ? "bg-success" : status === "connecting" ? "bg-warning animate-pulse" : "bg-destructive";
  const types = useMemo(() => {
    const m = new Map<string, number>();
    for (const r of rows) m.set(r.ev.type, (m.get(r.ev.type) ?? 0) + 1);
    return Array.from(m.entries()).sort((a, b) => b[1] - a[1]);
  }, [rows]);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageBody className="flex min-h-0 flex-1 flex-col gap-3 py-4" wide>
        <PageHeader
          title="Debug"
          meta={
            <span className="inline-flex items-center gap-1.5 text-[11px] text-muted-foreground">
              <span className={cn("size-1.5 rounded-full", dot)} />
              {supported ? `SSE ${status}` : "EventSource unsupported"}
              {session && (
                <Pill mono title="server-side session filter">
                  session={session}
                </Pill>
              )}
              {dropped > 0 && <span className="text-warning">{dropped} unparsable</span>}
            </span>
          }
          actions={
            <>
              <Button size="sm" variant={paused ? "subtle" : "outline"} onClick={() => setPaused((p) => !p)} aria-pressed={paused} title="Pause rendering (events keep buffering)">
                {paused ? <Play className="size-3.5" /> : <Pause className="size-3.5" />} {paused ? "Resume" : "Pause"}
              </Button>
              <Button size="sm" variant={autoScroll ? "subtle" : "outline"} onClick={() => setAutoScroll((a) => !a)} aria-pressed={autoScroll}>
                <ArrowDownToLine className="size-3.5" /> Auto-scroll
              </Button>
              <Button size="sm" variant="ghost" onClick={() => { clear(); setExpanded({}); }}>
                <Eraser className="size-3.5" /> Clear
              </Button>
            </>
          }
        />

        <div className="flex flex-wrap items-center gap-2">
          <input
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Filter by type or session…"
            aria-label="Filter events"
            className="h-8 w-64 max-w-full rounded-[10px] border border-input bg-background/60 px-3 text-[12.5px] outline-none placeholder:text-muted-foreground/50 focus-visible:border-primary/60 focus-visible:ring-2 focus-visible:ring-primary/25"
          />
          <form
            className="flex items-center gap-1.5"
            onSubmit={(e) => {
              e.preventDefault();
              clear();
              setExpanded({});
              setSession(sessionInput.trim());
            }}
          >
            <input
              value={sessionInput}
              onChange={(e) => setSessionInput(e.target.value)}
              placeholder="session id (server filter)"
              aria-label="Session id"
              className="h-8 w-52 max-w-full rounded-[10px] border border-input bg-background/60 px-3 font-mono text-[12px] tracking-normal outline-none placeholder:font-sans placeholder:tracking-[-0.02em] placeholder:text-muted-foreground/50 focus-visible:border-primary/60 focus-visible:ring-2 focus-visible:ring-primary/25"
            />
            <Button size="sm" type="submit">
              <Plug className="size-3.5" /> Reconnect
            </Button>
          </form>
          <div className="ml-auto flex flex-wrap gap-1">
            {types.slice(0, 8).map(([t, n]) => (
              <button
                key={t}
                type="button"
                onClick={() => setFilter((f) => (f === t ? "" : t))}
                className={cn(
                  "rounded-full border px-2 py-0.5 font-mono text-[10.5px] tracking-normal transition-colors active:scale-[0.97]",
                  filter === t ? "border-primary/40 bg-primary/[0.12] text-primary" : "border-border text-muted-foreground hover:bg-foreground/[0.04]",
                )}
              >
                {t} <span className="opacity-60">{n}</span>
              </button>
            ))}
          </div>
        </div>

        <div ref={listRef} className="min-h-[420px] flex-1 overflow-y-auto rounded-[15px] border border-border bg-card lg:min-h-0">
          {!supported ? (
            <EmptyState title="EventSource is not supported" hint="Use a modern browser to watch the live event stream." />
          ) : filtered.length === 0 ? (
            <EmptyState
              title={rows.length === 0 ? "Waiting for events" : "No events match the filter"}
              hint={rows.length === 0 ? "Talk to the robot on the Call page — every pipeline step streams here, including raw model tokens." : "Clear the filter to see everything."}
            />
          ) : (
            <ul>
              {filtered.map((r) => (
                <EventRow key={r.id} row={r} expanded={!!expanded[r.id]} onToggle={() => setExpanded((e) => ({ ...e, [r.id]: !e[r.id] }))} />
              ))}
            </ul>
          )}
        </div>
        <p className="text-[10.5px] text-muted-foreground/45">
          {rows.length} events buffered (max 600) · consecutive <span className="font-mono tracking-normal">llm_delta</span> tokens of one turn are merged into a growing row · <span className="font-mono tracking-normal">route</span> and{" "}
          <span className="font-mono tracking-normal">turn_done</span> are highlighted
        </p>
      </PageBody>
    </div>
  );
}
