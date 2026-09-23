"use client";

import { useEffect, useRef } from "react";
import { Bot, User } from "lucide-react";
import { cn } from "@/lib/cn";
import { fmtPct, fmtTime } from "@/lib/format";
import type { MicState, TurnEntry } from "@/lib/call-controller";
import { LangBadge, PathBadge } from "@/components/ui/pill";
import { EmptyState } from "@/components/ui/states";

export const SAMPLE_UTTERANCES = [
  "Сколько стоит ОГПО на машину в Астане?",
  "Сәлеметсіз бе, кеше аулада көлігімді біреу соғып кетіпті, КАСКО бар",
  "Что с моим заявлением по каско, когда будет решение?",
  "Мне звонили от вашего имени и просили код из СМС",
  "Хочу отменить полис и вернуть деньги",
];

export function Conversation({
  turns,
  partial,
  micState,
  selectedTurn,
  onSelect,
  onSend,
  className,
}: {
  turns: TurnEntry[];
  partial: string;
  micState: MicState;
  selectedTurn: number | null;
  onSelect: (turn: number) => void;
  onSend: (text: string) => void;
  className?: string;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const last = turns[turns.length - 1];
  const scrollKey = `${turns.length}:${last?.reply.length ?? 0}:${partial.length}:${micState}`;

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [scrollKey]);

  const selected = selectedTurn ?? last?.turn ?? null;

  return (
    <div ref={ref} className={cn("overflow-y-auto px-4 py-4", className)}>
      {turns.length === 0 && !partial ? (
        <EmptyState
          icon={<Bot className="size-4" />}
          title="Start a conversation"
          hint="Press the microphone or type a message. The robot answers in Russian or Kazakh and every decision shows up in the trace panel."
          className="py-6"
          action={
            <div className="flex max-w-lg flex-wrap justify-center gap-1.5">
              {SAMPLE_UTTERANCES.map((s) => (
                <button
                  key={s}
                  type="button"
                  onClick={() => onSend(s)}
                  className="rounded-full border border-border px-3 py-1 text-left text-[11.5px] text-muted-foreground transition-colors hover:border-primary/40 hover:bg-primary/[0.06] hover:text-foreground active:scale-[0.97]"
                >
                  {s}
                </button>
              ))}
            </div>
          }
        />
      ) : (
        <ol className="space-y-4">
          {turns.map((t) => {
            const isSel = selected === t.turn;
            const primary = t.trace.decision?.scenarios?.[0];
            const path = t.done ? t.trace.path : t.provisionalPath;
            return (
              <li key={t.turn} className="rise space-y-2">
                <button
                  type="button"
                  onClick={() => onSelect(t.turn)}
                  className="group ml-auto flex max-w-[85%] flex-col items-end gap-1 text-left"
                  aria-label={`Inspect turn ${t.turn}`}
                >
                  <div
                    className={cn(
                      "rounded-[14px] rounded-br-[4px] border px-3.5 py-2.5 text-[13.5px] leading-snug transition-colors",
                      isSel ? "border-primary/40 bg-primary/[0.14]" : "border-primary/20 bg-primary/[0.08] group-hover:border-primary/35",
                    )}
                  >
                    {t.transcript || <span className="text-muted-foreground/60">…</span>}
                  </div>
                  <div className="flex items-center gap-1.5 pr-1 text-[10.5px] text-muted-foreground/55">
                    <User className="size-3" />
                    <span>turn {t.turn}</span>
                    {t.source && t.source !== "text" && <span>· {t.source}</span>}
                    <span>· {fmtTime(t.at)}</span>
                  </div>
                </button>

                <button
                  type="button"
                  onClick={() => onSelect(t.turn)}
                  className="group flex max-w-[85%] flex-col items-start gap-1 text-left"
                  aria-label={`Inspect reply of turn ${t.turn}`}
                >
                  <div
                    className={cn(
                      "rounded-[14px] rounded-bl-[4px] border px-3.5 py-2.5 text-[13.5px] leading-snug transition-colors",
                      isSel ? "border-primary/35 bg-card" : "border-border bg-card group-hover:border-primary/25",
                      t.error && "border-destructive/30",
                    )}
                  >
                    {t.reply ? (
                      <span className={cn(!t.replyDone && !t.done && "caret")}>{t.reply}</span>
                    ) : t.error ? (
                      <span className="text-destructive">{t.error}</span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-muted-foreground/70">
                        <span className="size-1.5 animate-bounce rounded-full bg-current [animation-delay:-0.2s]" />
                        <span className="size-1.5 animate-bounce rounded-full bg-current [animation-delay:-0.1s]" />
                        <span className="size-1.5 animate-bounce rounded-full bg-current" />
                      </span>
                    )}
                  </div>
                  <div className="flex flex-wrap items-center gap-1.5 pl-1 text-[10.5px] text-muted-foreground/55">
                    <Bot className="size-3" />
                    {primary ? (
                      <>
                        <span className="font-mono tracking-normal text-foreground/70">{primary.id}</span>
                        <span>{fmtPct(primary.confidence)}</span>
                      </>
                    ) : (
                      <span>{t.done ? "no scenario" : "routing…"}</span>
                    )}
                    {path && <PathBadge path={path} provisional={!t.done} className="h-4 px-1.5 text-[9.5px]" />}
                    {t.trace.language?.reply && <LangBadge lang={t.trace.language.reply} className="h-4 px-1.5 text-[9.5px]" />}
                    {typeof t.client.e2e === "number" && <span>· e2e {t.client.e2e} ms</span>}
                  </div>
                </button>
              </li>
            );
          })}
          {partial && (
            <li className="ml-auto flex max-w-[85%] justify-end">
              <div className="rounded-[14px] rounded-br-[4px] border border-dashed border-primary/40 px-3.5 py-2.5 text-[13.5px] italic leading-snug text-foreground/70">
                {partial}
                <span className="caret" />
              </div>
            </li>
          )}
        </ol>
      )}
    </div>
  );
}
