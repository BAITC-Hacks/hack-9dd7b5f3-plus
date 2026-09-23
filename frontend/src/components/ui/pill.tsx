import type { ReactNode } from "react";
import { cn } from "@/lib/cn";
import { langLabel } from "@/lib/format";

export type Tone = "neutral" | "primary" | "success" | "warning" | "destructive" | "info";

export const toneClass: Record<Tone, string> = {
  neutral: "border-border bg-foreground/[0.04] text-muted-foreground",
  primary: "border-primary/30 bg-primary/10 text-primary",
  success: "border-success/30 bg-success/10 text-success",
  warning: "border-warning/30 bg-warning/10 text-warning",
  destructive: "border-destructive/30 bg-destructive/10 text-destructive",
  info: "border-info/30 bg-info/10 text-info",
};

export function Pill({
  tone = "neutral",
  mono,
  className,
  children,
  title,
}: {
  tone?: Tone;
  mono?: boolean;
  className?: string;
  children: ReactNode;
  title?: string;
}) {
  return (
    <span
      title={title}
      className={cn(
        "inline-flex h-5 shrink-0 items-center gap-1 rounded-full border px-2 text-[10.5px] font-medium leading-none whitespace-nowrap",
        mono && "font-mono tracking-normal",
        toneClass[tone],
        className,
      )}
    >
      {children}
    </span>
  );
}

export function LangBadge({ lang, className }: { lang?: string | null; className?: string }) {
  const l = (lang ?? "").toLowerCase();
  return (
    <Pill tone={l === "mixed" ? "primary" : "neutral"} className={className} title={l ? `language: ${l}` : "language unknown"}>
      {langLabel(l)}
    </Pill>
  );
}

const pathTone: Record<string, Tone> = { llm: "primary", fast: "success", mock: "neutral", fallback: "warning" };

export function PathBadge({ path, provisional, className }: { path?: string | null; provisional?: boolean; className?: string }) {
  const p = (path ?? "").toLowerCase();
  const tone = pathTone[p] ?? "neutral";
  return (
    <Pill tone={tone} mono className={className} title={provisional ? "provisional until turn_done" : `path: ${p || "?"}`}>
      {provisional && <span className="size-1.5 animate-pulse rounded-full bg-current" />}
      {p || "…"}
    </Pill>
  );
}

const policyTone: Record<string, Tone> = {
  proceed: "success",
  clarify: "warning",
  handoff: "info",
  out_of_scope: "neutral",
  goodbye: "neutral",
};

export function PolicyBadge({ action, className }: { action?: string | null; className?: string }) {
  const a = (action ?? "").toLowerCase();
  if (!a) return <Pill className={className}>—</Pill>;
  return (
    <Pill tone={policyTone[a] ?? "neutral"} className={className}>
      {a.replaceAll("_", " ")}
    </Pill>
  );
}

const modeTone: Record<string, Tone> = { execute: "success", preview: "warning", handoff: "info", cancelled: "neutral" };

export function ModeBadge({ mode }: { mode?: string | null }) {
  const m = (mode ?? "").toLowerCase();
  return <Pill tone={modeTone[m] ?? "neutral"}>{m || "—"}</Pill>;
}

export function ScenarioTag({ id, name, className }: { id: string; name?: string; className?: string }) {
  const tone: Tone = id.startsWith("SYS_") ? "neutral" : "primary";
  return (
    <Pill tone={tone} mono title={name ? `${id} — ${name}` : id} className={className}>
      {id}
    </Pill>
  );
}
