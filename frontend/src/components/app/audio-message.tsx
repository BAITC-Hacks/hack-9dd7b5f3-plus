"use client";
import { useUiLanguage, translate as t } from "@/lib/ui-language";

import { Download, Pause, Play, Volume2 } from "lucide-react";
import { useEffect, useState } from "react";
import { seekAudio, replayAudio, type SavedAudio } from "@/lib/voice";
import type { Message } from "@/lib/store";

const clock = (seconds: number) => `${Math.floor(seconds / 60)}:${Math.floor(seconds % 60).toString().padStart(2, "0")}`;

function Player({ saved, turn }: { saved: SavedAudio; turn: number }) {
  useUiLanguage();
  const audio = saved.element;
  const [playing, setPlaying] = useState(!audio.paused);
  const [time, setTime] = useState(audio.currentTime || 0);
  const [duration, setDuration] = useState(Number.isFinite(audio.duration) ? audio.duration : 0);
  const [peaks, setPeaks] = useState<number[]>(Array(48).fill(0.12));
  const [error, setError] = useState("");

  useEffect(() => {
    const update = () => {
      setPlaying(!audio.paused && !audio.ended);
      setTime(audio.currentTime || 0);
      setDuration(Number.isFinite(audio.duration) ? audio.duration : 0);
    };
    const events = ["play", "pause", "ended", "timeupdate", "loadedmetadata", "durationchange", "seeked"];
    events.forEach((event) => audio.addEventListener(event, update));
    return () => events.forEach((event) => audio.removeEventListener(event, update));
  }, [audio]);

  useEffect(() => {
    let cancelled = false;
    const context = new AudioContext();
    void (async () => {
      try {
        const blob = saved.blob ?? await (await fetch(saved.url)).blob();
        const decoded = await context.decodeAudioData(await blob.arrayBuffer());
        const samples = decoded.getChannelData(0);
        const values = Array.from({ length: 48 }, (_, i) => {
          const start = Math.floor(i * samples.length / 48);
          const end = Math.floor((i + 1) * samples.length / 48);
          let sum = 0;
          for (let j = start; j < end; j++) sum += samples[j] * samples[j];
          return Math.sqrt(sum / Math.max(1, end - start));
        });
        const max = Math.max(...values, 0.001);
        if (!cancelled) { setPeaks(values.map((v) => Math.max(0.12, v / max))); setDuration(decoded.duration); }
      } catch { /* Playback and seeking remain usable if waveform decoding is unsupported. */ }
      finally { if (context.state !== "closed") await context.close(); }
    })();
    return () => { cancelled = true; if (context.state !== "closed") void context.close(); };
  }, [saved]);

  const progress = duration ? time / duration : 0;
  return <div className="min-w-0">
    <div className="flex items-center gap-3">
      <button type="button" aria-label={playing ? t("Приостановить ответ") : t("Воспроизвести ответ")}
        className="grid size-10 shrink-0 place-items-center rounded-full bg-primary text-primary-foreground focus-visible:outline-2 focus-visible:outline-offset-2"
        onClick={async () => {
          setError("");
          if (!audio.paused) audio.pause();
          else try { await replayAudio(saved); } catch { setError(t("Не удалось включить звук. Нажмите Play ещё раз.")); }
        }}>{playing ? <Pause className="size-4" /> : <Play className="size-4" />}</button>
      <div className="relative h-10 min-w-0 flex-1">
        <div aria-hidden="true" className="flex h-full items-center gap-[3px]">
          {peaks.map((peak, i) => <span key={i} className={`min-w-0 flex-1 rounded-full transition-colors duration-150 ${i / peaks.length < progress ? "bg-primary" : "bg-muted-foreground/25"}`}
            style={{ height: `${peak * 100}%` }} />)}
        </div>
        <input type="range" min={0} max={duration || 1} step={0.01} value={Math.min(time, duration || 1)} disabled={!duration}
          aria-label={t("Перемотка аудиоответа")} aria-valuetext={`${clock(time)} из ${clock(duration)}`}
          className="absolute inset-0 h-full w-full cursor-pointer opacity-0 focus-visible:opacity-70"
          onChange={(event) => { seekAudio(saved, Number(event.target.value)); setTime(audio.currentTime); }} />
      </div>
      <a href={saved.url} download={`bagyt-answer-${turn}.mp3`} aria-label={t("Скачать аудиоответ")} className="shrink-0 text-muted-foreground hover:text-foreground"><Download className="size-4" /></a>
    </div>
    <div className="mt-1 flex justify-between pl-[52px] text-[11px] tabular-nums text-muted-foreground"><span>{playing ? t("Воспроизводится") : t("Аудиоответ")}</span><span>{clock(time)} / {clock(duration)}</span></div>
    {error && <p role="alert" className="mt-2 text-xs text-destructive">{error}</p>}
  </div>;
}

export function AudioMessage({ message }: { message: Message }) {
  useUiLanguage();
  return <div className="w-[320px] max-w-full">
    {message.audio ? <Player saved={message.audio} turn={message.turn} /> : <div className="flex items-center gap-2 py-2 text-xs text-muted-foreground"><Volume2 className="size-4" />{message.audioState === "unavailable" ? t("Запись недоступна") : t("Готовлю аудиоответ…")}</div>}
    <div className="mt-3 border-t border-border/60 pt-3">
      <div className="mb-1 text-[10px] uppercase tracking-wider text-muted-foreground">{t("Транскрипция")}</div>
      <p className="text-sm leading-relaxed text-muted-foreground">{message.text}{message.streaming && <span className="ml-1 animate-pulse">▍</span>}</p>
    </div>
  </div>;
}
