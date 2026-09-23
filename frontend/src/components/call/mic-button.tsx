"use client";

import { useSyncExternalStore } from "react";
import { Loader2, Mic, MicOff, Volume2 } from "lucide-react";
import { cn } from "@/lib/cn";
import type { MicMode, MicState } from "@/lib/call-controller";

const zero = () => 0;

export function MicButton({
  state,
  armed,
  inSpeech,
  mode,
  disabled,
  subscribeLevel,
  getLevel,
  onToggle,
  onPttStart,
  onPttEnd,
  onStop,
}: {
  state: MicState;
  armed: boolean;
  inSpeech: boolean;
  mode: MicMode;
  disabled?: boolean;
  subscribeLevel: (l: () => void) => () => void;
  getLevel: () => number;
  onToggle: () => void;
  onPttStart: () => void;
  onPttEnd: () => void;
  onStop: () => void;
}) {
  const level = useSyncExternalStore(subscribeLevel, getLevel, zero);
  const listening = state === "listening";
  const thinking = state === "thinking";
  const speaking = state === "speaking";
  const active = listening && (inSpeech || armed || mode === "ptt");

  const label =
    mode === "ptt"
      ? inSpeech
        ? "Release to send"
        : speaking
          ? "Speaking — hold to interrupt"
          : thinking
            ? "Thinking…"
            : "Hold to talk (or Space)"
      : inSpeech
        ? "Listening…"
        : speaking
          ? "Speaking — click to stop"
          : thinking
            ? "Thinking…"
            : armed
              ? "Waiting for speech — click to stop"
              : "Click to start listening";

  const handleClick = () => {
    if (mode !== "auto") return;
    if (speaking) {
      onStop();
      return;
    }
    onToggle();
  };

  const ring = inSpeech || (listening && armed && mode === "auto");
  const ringScale = 1 + Math.min(1, level) * 0.75;

  return (
    <div className="flex flex-col items-center gap-2">
      <div className="relative">
        {ring && (
          <span
            aria-hidden
            className={cn("absolute inset-0 rounded-full border-2 border-primary/50 transition-transform duration-75", !inSpeech && "ring-pulse")}
            style={inSpeech ? { transform: `scale(${ringScale})`, opacity: 0.35 + Math.min(1, level) * 0.6 } : undefined}
          />
        )}
        {speaking && <span aria-hidden className="ring-pulse absolute inset-0 rounded-full border-2 border-success/50" />}
        <button
          type="button"
          disabled={disabled}
          aria-label={label}
          title={label}
          aria-pressed={armed || inSpeech}
          onClick={handleClick}
          onPointerDown={(e) => {
            if (mode !== "ptt") return;
            e.preventDefault();
            e.currentTarget.setPointerCapture?.(e.pointerId);
            onPttStart();
          }}
          onPointerUp={() => mode === "ptt" && onPttEnd()}
          onPointerCancel={() => mode === "ptt" && onPttEnd()}
          onPointerLeave={() => mode === "ptt" && inSpeech && onPttEnd()}
          onContextMenu={(e) => mode === "ptt" && e.preventDefault()}
          className={cn(
            "relative grid size-[72px] select-none place-items-center rounded-full border-2 transition-[background-color,border-color,color,transform] duration-150 active:scale-[0.96] disabled:opacity-40 touch-none",
            speaking
              ? "border-success/50 bg-success/10 text-success"
              : thinking
                ? "border-primary/40 bg-primary/[0.08] text-primary"
                : active
                  ? "border-primary bg-primary text-primary-foreground"
                  : "border-border bg-card text-muted-foreground hover:border-primary/40 hover:text-foreground",
          )}
        >
          {thinking ? (
            <Loader2 className="size-7 animate-spin" strokeWidth={1.75} />
          ) : speaking ? (
            <Volume2 className="size-7" strokeWidth={1.75} />
          ) : active ? (
            <Mic className="size-7" strokeWidth={1.75} />
          ) : (
            <MicOff className="size-7" strokeWidth={1.75} />
          )}
        </button>
      </div>
      <p className="h-4 text-center text-[11px] text-muted-foreground">{label}</p>
    </div>
  );
}
