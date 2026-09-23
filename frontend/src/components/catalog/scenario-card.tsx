"use client";

import { Headset, IdCard, ShieldCheck, Zap } from "lucide-react";
import { cn } from "@/lib/cn";
import type { Scenario } from "@/lib/types";
import { Collapsible } from "@/components/ui/collapsible";
import { Label } from "@/components/ui/panel";
import { Pill, type Tone } from "@/components/ui/pill";

const priorityTone: Record<string, Tone> = { urgent: "destructive", high: "warning", normal: "neutral" };

function Flag({ on, icon, label, title }: { on: boolean; icon: React.ReactNode; label: string; title: string }) {
  return (
    <span
      title={title}
      className={cn(
        "inline-flex h-5 items-center gap-1 rounded-full border px-1.5 text-[10.5px]",
        on ? "border-primary/30 bg-primary/10 text-primary" : "border-border text-muted-foreground/35",
      )}
    >
      {icon}
      {label}
    </span>
  );
}

export function ScenarioCard({ s, names, highlight }: { s: Scenario; names: Record<string, string>; highlight?: string }) {
  const ru = s.examples?.ru ?? [];
  const kk = s.examples?.kk ?? [];
  const req = s.slots?.required ?? [];
  const opt = s.slots?.optional ?? [];
  const actions = s.actions ?? [];
  const rules = s.not_this_if ?? [];
  return (
    <article
      id={s.scenario_id}
      className={cn("target-highlight rise scroll-mt-4 rounded-[15px] border border-border bg-card p-4 transition-[border-color,box-shadow]", highlight === s.scenario_id && "border-primary/50")}
    >
      <header className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="font-mono text-[12px] tracking-normal text-primary">{s.scenario_id}</span>
            <span className="text-[10.5px] text-muted-foreground/50">{s.slug}</span>
          </div>
          <h3 className="mt-0.5 text-[14.5px] font-[450] leading-snug tracking-[-0.02em]">{s.name}</h3>
        </div>
        <div className="flex flex-wrap items-center gap-1">
          <Pill>{s.domain}</Pill>
          <Pill>{s.category}</Pill>
          <Pill tone={priorityTone[s.priority] ?? "neutral"}>{s.priority}</Pill>
        </div>
      </header>
      <p className="mt-2 text-[12.5px] leading-relaxed text-foreground/80">{s.description}</p>

      <div className="mt-2.5 flex flex-wrap gap-1.5">
        <Flag on={s.fast_path_eligible} icon={<Zap className="size-3" />} label="fast path" title="Eligible for the lexical fast path (no LLM when the shortlist is obvious)" />
        <Flag on={s.requires_identification} icon={<IdCard className="size-3" />} label="identification" title="Client must be identified (phone / IIN)" />
        <Flag on={s.requires_confirmation} icon={<ShieldCheck className="size-3" />} label="confirmation" title="Irreversible action needs an explicit yes" />
        <Flag on={!!s.handoff} icon={<Headset className="size-3" />} label={s.handoff ? `handoff → ${s.handoff.queue}` : "handoff"} title={s.handoff ? `Handoff when: ${s.handoff.when}` : "No human handoff"} />
      </div>

      {rules.length > 0 && (
        <div className="mt-3 space-y-1">
          <Label>not this if</Label>
          <ul className="space-y-1">
            {rules.map((b, i) => (
              <li key={i} className="flex flex-wrap items-baseline gap-x-2 text-[12px] leading-snug text-foreground/80">
                <span>{b.condition}</span>
                <a
                  href={`#${b.use_instead}`}
                  className="inline-flex items-center gap-1 font-mono text-[11px] tracking-normal text-primary hover:underline"
                  title={names[b.use_instead] ?? b.use_instead}
                >
                  → {b.use_instead}
                  {names[b.use_instead] && <span className="font-sans tracking-[-0.02em] text-muted-foreground/70">{names[b.use_instead]}</span>}
                </a>
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <div className="space-y-1">
          <Label>slots</Label>
          <div className="flex flex-wrap gap-1">
            {req.map((sl) => (
              <Pill key={`r-${sl}`} tone="primary" mono title="required">
                {sl}
              </Pill>
            ))}
            {opt.map((sl) => (
              <Pill key={`o-${sl}`} mono title="optional">
                {sl}
              </Pill>
            ))}
            {req.length + opt.length === 0 && <span className="text-[12px] text-muted-foreground/40">none</span>}
          </div>
        </div>
        <div className="space-y-1">
          <Label>actions</Label>
          <div className="flex flex-wrap gap-1">
            {actions.map((a) => (
              <Pill key={a} mono>
                {a}
              </Pill>
            ))}
            {actions.length === 0 && <span className="text-[12px] text-muted-foreground/40">none</span>}
          </div>
        </div>
      </div>

      <div className="mt-3 space-y-1.5">
        <Collapsible title="examples" count={ru.length + kk.length} size="sm">
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <Label className="mb-1">RU</Label>
              <ul className="space-y-1 text-[12px] leading-snug text-foreground/85">
                {ru.map((e, i) => (
                  <li key={i}>“{e}”</li>
                ))}
              </ul>
            </div>
            <div>
              <Label className="mb-1">KK</Label>
              <ul className="space-y-1 text-[12px] leading-snug text-foreground/85">
                {kk.map((e, i) => (
                  <li key={i}>“{e}”</li>
                ))}
              </ul>
            </div>
          </div>
        </Collapsible>
        {s.responses && (
          <Collapsible title="responses" size="sm">
            <div className="space-y-2">
              {Object.entries(s.responses).map(([lang, r]) => (
                <div key={lang} className="text-[12px] leading-snug">
                  <Label className="mb-0.5">{lang.toUpperCase()}</Label>
                  <p className="text-foreground/85">
                    <span className="text-muted-foreground/60">opening · </span>
                    {r.opening}
                  </p>
                  <p className="text-foreground/85">
                    <span className="text-muted-foreground/60">closing · </span>
                    {r.closing}
                  </p>
                </div>
              ))}
            </div>
          </Collapsible>
        )}
      </div>
    </article>
  );
}
