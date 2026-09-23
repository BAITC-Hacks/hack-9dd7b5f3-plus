"use client";

import { RotateCcw, Volume2, VolumeX } from "lucide-react";
import { cn } from "@/lib/cn";
import type { BrowserLang, CallState, MicMode, SttSource } from "@/lib/call-controller";
import { Button } from "@/components/ui/button";
import { Segmented } from "@/components/ui/segmented";

export function CallControls({
  st,
  onMode,
  onStt,
  onLang,
  onMuted,
  onBargeIn,
  onSensitivity,
  onNewSession,
}: {
  st: CallState;
  onMode: (m: MicMode) => void;
  onStt: (s: SttSource) => void;
  onLang: (l: BrowserLang) => void;
  onMuted: (m: boolean) => void;
  onBargeIn: (b: boolean) => void;
  onSensitivity: (v: number) => void;
  onNewSession: () => void;
}) {
  const serverHint = st.serverSttAvailable ? `server STT: ${st.config?.stt.provider ?? ""}` : "server STT not configured — using browser STT";
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2 text-[11.5px] text-muted-foreground">
      <div className="flex items-center gap-2">
        <span className="text-[10.5px] text-muted-foreground/55">mode</span>
        <Segmented<MicMode>
          ariaLabel="Microphone mode"
          size="sm"
          value={st.mode}
          onChange={onMode}
          options={[
            { value: "auto", label: "Auto (VAD)", title: "Hands-free: voice activity detection starts and ends each utterance" },
            { value: "ptt", label: "Push-to-talk", title: "Hold the mic button (or Space) while speaking" },
          ]}
        />
      </div>
      <div className="flex items-center gap-2">
        <span className="text-[10.5px] text-muted-foreground/55">stt</span>
        <Segmented<SttSource>
          ariaLabel="Speech recognition source"
          size="sm"
          value={st.sttSource}
          onChange={onStt}
          options={[
            { value: "server", label: "Server", disabled: !st.serverSttAvailable, title: serverHint },
            { value: "browser", label: "Browser", disabled: !st.browserSttAvailable, title: st.browserSttAvailable ? "Web Speech API (Chrome)" : "Web Speech API is not available in this browser" },
          ]}
        />
        {st.sttSource === "browser" && (
          <select
            aria-label="Browser STT language"
            value={st.browserLang}
            onChange={(e) => onLang(e.target.value as BrowserLang)}
            className="h-6 rounded-[8px] border border-border bg-muted px-1.5 text-[11.5px] text-foreground outline-none focus-visible:ring-2 focus-visible:ring-primary/40"
          >
            <option value="ru-RU">ru-RU</option>
            <option value="kk-KZ">kk-KZ</option>
          </select>
        )}
      </div>
      <div className="flex items-center gap-1.5">
        <Button size="sm" variant={st.muted ? "danger" : "ghost"} onClick={() => onMuted(!st.muted)} aria-pressed={st.muted} title={st.muted ? "Unmute the robot" : "Mute the robot"}>
          {st.muted ? <VolumeX className="size-3.5" /> : <Volume2 className="size-3.5" />}
          {st.muted ? "Muted" : "Sound"}
        </Button>
        {st.mode === "auto" && (
          <Button
            size="sm"
            variant={st.bargeIn ? "subtle" : "ghost"}
            onClick={() => onBargeIn(!st.bargeIn)}
            aria-pressed={st.bargeIn}
            title="Let the client interrupt the robot while it speaks (off by default to avoid echo triggering)"
          >
            Barge-in {st.bargeIn ? "on" : "off"}
          </Button>
        )}
        {st.mode === "auto" && st.sttSource === "server" && (
          <label className={cn("flex items-center gap-1.5", !st.armed && "opacity-70")} title="VAD sensitivity">
            <span className="text-[10.5px] text-muted-foreground/55">sensitivity</span>
            <input
              type="range"
              min={0}
              max={100}
              value={Math.round(st.sensitivity * 100)}
              onChange={(e) => onSensitivity(Number(e.target.value) / 100)}
              className="h-1 w-20 accent-primary"
              aria-label="VAD sensitivity"
            />
          </label>
        )}
      </div>
      <Button size="sm" variant="outline" onClick={onNewSession} className="ml-auto" title="Start a fresh session">
        <RotateCcw className="size-3.5" /> New session
      </Button>
      {!st.serverSttAvailable && st.config && (
        <p className="basis-full text-[10.5px] leading-snug text-muted-foreground/55" title={serverHint}>
          server STT not configured — using browser STT
        </p>
      )}
    </div>
  );
}
