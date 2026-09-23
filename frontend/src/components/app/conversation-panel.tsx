"use client";
/**
 * Shared conversation widget: chat feed + push-to-talk mic + text fallback.
 * Used by /call (client simulator) and /admin (supervisor console, left column).
 */
import { Mic, Send, Square, Volume2, VolumeX } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { sendText, setTts, startVoice, stopVoice, useConversation, type Message } from "@/lib/store";

const LANG_LABEL: Record<string, string> = { ru: "RU", kk: "KK", mixed: "RU+KK" };

export function LangBadge({ lang, className }: { lang?: string; className?: string }) {
  if (!lang) return null;
  const variant = lang === "mixed" ? "info" : lang === "kk" ? "success" : "secondary";
  return (
    <Badge variant={variant} size="sm" className={cn("font-mono tracking-[.08em]", className)}>
      {LANG_LABEL[lang] ?? lang.toUpperCase()}
    </Badge>
  );
}

function Bubble({ m }: { m: Message }) {
  const isBot = m.role === "bot";
  return (
    <div className={cn("flex w-full gap-3", isBot ? "justify-start" : "justify-end")}>
      <div className={cn("max-w-[78%] rounded-xl border px-3.5 py-2.5 text-[15px] leading-relaxed sm:text-sm", isBot ? "border-border bg-card text-card-foreground" : "border-transparent bg-primary text-primary-foreground")}>
        <div className="mb-1 flex items-center gap-2">
          <span className={cn("marker !text-[10px]", !isBot && "!text-primary-foreground/70")}>{isBot ? "Bagyt" : "Клиент"} · #{m.turn}</span>
          {m.lang && <LangBadge lang={m.lang} className={cn(!isBot && "bg-white/15 text-white")} />}
        </div>
        <div className={cn(m.streaming && "after:ml-0.5 after:inline-block after:h-3.5 after:w-[2px] after:animate-pulse after:bg-current after:align-middle")}>{m.text}</div>
      </div>
    </div>
  );
}

export function ConversationPanel({ className, compact = false }: { className?: string; compact?: boolean }) {
  const s = useConversation();
  const [text, setText] = useState("");
  const feedRef = useRef<HTMLDivElement>(null);
  const holding = useRef(false);

  useEffect(() => {
    feedRef.current?.scrollTo({ top: feedRef.current.scrollHeight, behavior: "smooth" });
  }, [s.messages.length, s.interim, s.live?.responseText]);

  const listening = s.status === "listening";
  const thinking = s.status === "thinking";

  const onMicDown = () => { holding.current = true; void startVoice(); };
  const onMicUp = () => { if (!holding.current) return; holding.current = false; void stopVoice(); };

  const submit = () => {
    const t = text.trim();
    if (!t) return;
    setText("");
    void sendText(t);
  };

  return (
    <div className={cn("flex h-full min-h-0 flex-col", className)}>
      {/* feed */}
      <div ref={feedRef} className="flex-1 min-h-0 space-y-3 overflow-y-auto px-4 py-4 sm:px-5">
        {s.messages.length === 0 && !s.interim && (
          <div className="flex h-full flex-col items-center justify-center gap-2 text-center">
            <div className="grid size-10 place-items-center rounded-lg border border-border bg-card"><Mic className="size-4 opacity-70" /></div>
            <div className="text-sm font-medium">Скажите, что случилось</div>
            <div className="max-w-xs text-xs text-muted-foreground">Зажмите кнопку микрофона и говорите по-русски, по-казахски или вперемешку. Текст — резервный канал.</div>
          </div>
        )}
        {s.messages.map((m) => <Bubble key={m.id} m={m} />)}
        {s.interim && (
          <div className="flex w-full justify-end">
            <div className="max-w-[78%] rounded-xl border border-dashed border-primary/40 px-3.5 py-2.5 text-sm text-muted-foreground">{s.interim}<span className="ml-0.5 inline-block h-3.5 w-[2px] animate-pulse bg-current align-middle" /></div>
          </div>
        )}
        {thinking && !s.live?.responseText && (
          <div className="flex items-center gap-2 text-xs text-muted-foreground"><span className="size-1.5 animate-pulse rounded-full bg-brand" /> маршрутизирую…</div>
        )}
      </div>

      {/* composer */}
      <div className="hairline-t px-4 py-3 sm:px-5">
        {s.error && <div className="mb-2 rounded-md border border-destructive/30 bg-destructive/8 px-3 py-2 text-xs text-destructive-foreground">{s.error}</div>}
        <div className="flex items-center gap-2">
          <Button
            variant={listening ? "destructive" : "default"}
            size={compact ? "icon-lg" : "icon-xl"}
            aria-label={listening ? "Отпустите, чтобы отправить" : "Зажмите и говорите"}
            className={cn("shrink-0 select-none rounded-full", listening && "animate-pulse")}
            onPointerDown={onMicDown}
            onPointerUp={onMicUp}
            onPointerLeave={onMicUp}
            onPointerCancel={onMicUp}
            disabled={thinking}
          >
            {listening ? <Square /> : <Mic />}
          </Button>
          <Input
            placeholder={listening ? "Слушаю…" : "Или напишите текстом…"}
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") submit(); }}
            disabled={thinking || listening}
            className="flex-1"
          />
          <Button variant="secondary" size="icon" aria-label="Отправить" onClick={submit} disabled={!text.trim() || thinking}><Send /></Button>
          <Button variant="ghost" size="icon" aria-label={s.tts ? "Выключить озвучку" : "Включить озвучку"} onClick={() => setTts(!s.tts)}>{s.tts ? <Volume2 /> : <VolumeX />}</Button>
        </div>
        <div className="mt-2 flex items-center justify-between">
          <span className="marker !text-[10px]">{listening ? "● запись" : thinking ? "● обработка" : s.status === "speaking" ? "● говорит" : "○ готов"}</span>
          {!s.sttAvailable && s.sessionId && <span className="text-[10px] text-muted-foreground">Распознавание речи: только Chrome</span>}
        </div>
      </div>
    </div>
  );
}
