"use client";
import { useUiLanguage, translate as t } from "@/lib/ui-language";
/**
 * Shared conversation widget: chat feed + microphone (click to start, pause or click to send) + text input.
 * Used by /call (client) and /admin (supervisor, left column).
 */
import { AudioMessage } from "./audio-message";
import { Mic, SendHorizontal, Square, Volume2, VolumeX } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { sendText, setTts, toggleVoice, useConversation, type Message } from "@/lib/store";

const LANG_LABEL: Record<string, string> = { ru: "RU", kk: "KK", mixed: "RU+KK" };

export function LangBadge({ lang, className }: { lang?: string; className?: string }) {
  useUiLanguage();
  if (!lang) return null;
  const variant = lang === "mixed" ? "info" : lang === "kk" ? "success" : "secondary";
  return (
    <Badge variant={variant} size="sm" className={cn("font-mono", className)}>
      {LANG_LABEL[lang] ?? lang.toUpperCase()}
    </Badge>
  );
}

function Bubble({ m }: { m: Message }) {
  useUiLanguage();
  const isBot = m.role === "bot";
  return (
    <div className={cn("flex w-full", isBot ? "justify-start" : "justify-end")}>
      <div className={cn("max-w-[80%] rounded-2xl px-4 py-2.5 text-[15px] leading-relaxed", isBot ? "bg-card text-card-foreground" : "bg-primary text-primary-foreground")}>
        {isBot ? <AudioMessage message={m} /> : <div className={cn(m.streaming && "after:ml-0.5 after:inline-block after:h-3.5 after:w-[2px] after:animate-pulse after:bg-current after:align-middle")}>{m.text}</div>}
      </div>
    </div>
  );
}

const STATUS_TEXT = {
  idle: "Нажмите на микрофон и говорите",
  listening: "Слушаю… говорите, отвечу после паузы",
  thinking: "Думаю…",
  speaking: "Отвечаю",
} as const;

export function ConversationPanel({ className, compact = false }: { className?: string; compact?: boolean }) {
  useUiLanguage();
  const s = useConversation();
  const [text, setText] = useState("");
  const feedRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    feedRef.current?.scrollTo({ top: feedRef.current.scrollHeight, behavior: "smooth" });
  }, [s.messages.length, s.interim, s.live?.responseText]);

  const listening = s.status === "listening";
  const thinking = s.status === "thinking";

  const submit = () => {
    const t = text.trim();
    if (!t) return;
    setText("");
    void sendText(t);
  };

  return (
    <div className={cn("flex h-full min-h-0 flex-col", className)}>
      <div ref={feedRef} className="flex-1 min-h-0 space-y-2.5 overflow-y-auto px-4 py-4 sm:px-5">
        {s.messages.length === 0 && !s.interim && (
          <div className="flex h-full flex-col items-center justify-center gap-2 text-center">
            <div className="text-[15px]">{t("Скажите, что случилось")}</div>
            <div className="max-w-xs text-sm text-muted-foreground">{t("По-русски, по-казахски или вперемешку. Например: «Хочу продлить полис» или «Кеше аварияға түстім».")}</div>
          </div>
        )}
        {s.messages.map((m) => <Bubble key={m.id} m={m} />)}
        {s.interim && (
          <div className="flex w-full justify-end">
            <div className="max-w-[80%] rounded-2xl border border-dashed border-primary/40 px-4 py-2.5 text-[15px] text-muted-foreground">{s.interim}</div>
          </div>
        )}
        {thinking && !s.live?.responseText && (
          <div className="flex items-center gap-2 text-sm text-muted-foreground"><span className="size-1.5 animate-pulse rounded-full bg-brand" />{t("думаю…")}</div>
        )}
      </div>

      <div className="border-t border-border px-4 py-3 sm:px-5">
        {s.error && <div className="mb-2 rounded-lg border border-destructive/30 bg-destructive/8 px-3 py-2 text-sm text-destructive-foreground">{s.error}</div>}
        {s.notice && !s.error && <div className="mb-2 rounded-lg bg-muted px-3 py-2 text-sm text-muted-foreground">{s.notice}</div>}
        <div className="flex items-center gap-2">
          <Button
            variant={listening ? "destructive" : "default"}
            size={compact ? "icon-lg" : "icon-xl"}
            aria-label={listening ? t("Отправить") : t("Говорить")}
            aria-pressed={listening}
            className={cn("shrink-0 rounded-full transition-shadow duration-100")}
            style={listening ? { boxShadow: `0 0 0 ${4 + Math.round(s.level * 16)}px rgba(47,106,209,0.28)` } : undefined}
            onClick={() => void toggleVoice()}
            disabled={thinking}
          >
            {listening ? <Square /> : <Mic />}
          </Button>
          <Input
            placeholder={t("Или напишите текстом")}
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") submit(); }}
            disabled={thinking || listening}
            className="flex-1"
          />
          <Button variant="secondary" size="icon" aria-label={t("Отправить")} onClick={submit} disabled={!text.trim() || thinking}><SendHorizontal /></Button>
          <Button variant="ghost" size="icon" aria-label={s.tts ? t("Выключить озвучку") : t("Включить озвучку")} onClick={() => setTts(!s.tts)}>{s.tts ? <Volume2 /> : <VolumeX />}</Button>
        </div>
        <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
          <span className={cn(listening && "text-foreground")}>{t(STATUS_TEXT[s.status])}</span>
          {s.sessionId && s.sttProvider === "browser" && !s.sttAvailable && <span>{t("Голос работает в Chrome")}</span>}
        </div>
      </div>
    </div>
  );
}
