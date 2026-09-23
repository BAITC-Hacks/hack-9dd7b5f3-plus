"use client";

import type { ReactNode } from "react";
import { AlertCircle, ArrowRight, Check, X } from "lucide-react";
import { cn } from "@/lib/cn";
import { fmtMs, fmtPct, fmtTime, langLabel } from "@/lib/format";
import type { ScenarioNames } from "@/lib/catalog-cache";
import type { ActionRecord, Candidate, LiveTrace, TraceView } from "@/lib/types";
import { Collapsible } from "@/components/ui/collapsible";
import { ConfidenceBar, DEFAULT_THRESHOLDS, type Thresholds } from "@/components/ui/confidence-bar";
import { JsonView } from "@/components/ui/json-view";
import { KV, valueText } from "@/components/ui/kv";
import { Label, Section } from "@/components/ui/panel";
import { LangBadge, ModeBadge, PathBadge, Pill, PolicyBadge, ScenarioTag } from "@/components/ui/pill";
import { TimingsWaterfall } from "./timings-waterfall";

export interface TracePanelProps {
  view: TraceView;
  names: ScenarioNames;
  thresholds?: Thresholds;
  compact?: boolean;
  showReply?: boolean;
  className?: string;
}

function Empty({ children }: { children: ReactNode }) {
  return <p className="text-[12px] text-muted-foreground/45">{children}</p>;
}

function Chip({ children, tone = "neutral", title }: { children: ReactNode; tone?: "neutral" | "warning" | "destructive" | "info" | "success" | "primary"; title?: string }) {
  return (
    <Pill tone={tone} title={title} className="h-auto min-h-5 whitespace-normal py-0.5 text-[10.5px]">
      {children}
    </Pill>
  );
}

function ScenarioCards({ trace, names, thresholds, done }: { trace: LiveTrace; names: ScenarioNames; thresholds: Thresholds; done: boolean }) {
  const picks = trace.decision?.scenarios ?? [];
  const alts = trace.decision?.alternatives ?? [];
  if (picks.length === 0) {
    return done ? <Empty>no scenario chosen</Empty> : <p className="text-[12px] text-muted-foreground/55 caret">routing</p>;
  }
  return (
    <div className="space-y-2">
      {picks.map((p, i) => (
        <div key={`${p.id}-${i}`} className={cn("rounded-[12px] border px-3 py-2.5", i === 0 ? "border-primary/30 bg-primary/[0.06]" : "border-border bg-foreground/[0.02]")}>
          <div className="flex items-center justify-between gap-2">
            <div className="flex min-w-0 items-center gap-2">
              <ScenarioTag id={p.id} name={names[p.id]} />
              <span className="truncate text-[12.5px] text-foreground/90">{names[p.id] ?? (p.id.startsWith("SYS_") ? p.id.replace("SYS_", "system: ").toLowerCase() : "unknown scenario")}</span>
            </div>
            {i === 0 && <span className="shrink-0 text-[10px] text-muted-foreground/55">primary</span>}
          </div>
          <ConfidenceBar value={p.confidence} thresholds={thresholds} className="mt-2" width="w-full max-w-[160px]" />
          {p.reason && <p className="mt-1.5 text-[11.5px] leading-snug text-muted-foreground">{p.reason}</p>}
        </div>
      ))}
      {alts.length > 0 && (
        <div className="space-y-1 pt-0.5">
          <Label>alternatives</Label>
          {alts.map((a, i) => (
            <div key={`${a.id}-${i}`} className="flex items-center gap-2 text-[11.5px]">
              <ScenarioTag id={a.id} name={names[a.id]} />
              <span className="min-w-0 flex-1 truncate text-muted-foreground">{names[a.id] ?? ""}</span>
              <ConfidenceBar value={a.confidence} thresholds={thresholds} width="w-12" />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function SignalChips({ trace }: { trace: LiveTrace }) {
  const s = trace.triage;
  if (!s) return <Empty>waiting for triage</Empty>;
  const chips: ReactNode[] = [];
  for (const [k, v] of Object.entries(s.entities ?? {})) {
    chips.push(
      <Chip key={`e-${k}`} tone="info" title="entity">
        {k}: <span className="font-mono">{v}</span>
      </Chip>,
    );
  }
  for (const u of s.urgent ?? []) chips.push(<Chip key={`u-${u}`} tone="destructive" title="urgent marker">urgent: {u}</Chip>);
  for (const m of s.multi_intent_markers ?? []) chips.push(<Chip key={`m-${m}`} tone="warning" title="multi-intent marker">multi-intent: {m}</Chip>);
  if (s.confirmation) chips.push(<Chip key="conf" tone={s.confirmation === "yes" ? "success" : "warning"}>confirmation: {s.confirmation}</Chip>);
  if (s.goodbye) chips.push(<Chip key="bye">goodbye</Chip>);
  if (s.greeting_only) chips.push(<Chip key="greet">greeting only</Chip>);
  if (s.operator_request) chips.push(<Chip key="op" tone="info">operator request</Chip>);
  if (s.robot_question) chips.push(<Chip key="rq">asks if robot</Chip>);
  for (const h of s.out_of_scope_hints ?? []) chips.push(<Chip key={`o-${h}`}>out of scope: {h}</Chip>);
  if (typeof s.kk_share === "number" && s.kk_share > 0) chips.push(<Chip key="kk">kk share {fmtPct(s.kk_share)}</Chip>);
  if (chips.length === 0) return <Empty>no signals</Empty>;
  return <div className="flex flex-wrap gap-1.5">{chips}</div>;
}

function RetrievalList({ items, names }: { items: Candidate[]; names: ScenarioNames }) {
  const top = Math.max(0.001, ...items.map((c) => c.score));
  return (
    <ul className="space-y-1.5">
      {items.map((c) => (
        <li key={c.id} className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-x-2 gap-y-0.5 text-[11.5px]">
          <ScenarioTag id={c.id} name={names[c.id]} />
          <span className="truncate text-muted-foreground">{names[c.id] ?? ""}</span>
          <span className="tabular-nums text-muted-foreground" title={`bm25 ${c.bm25}`}>
            {c.score.toFixed(3)}
          </span>
          <div className="col-span-3 h-1 overflow-hidden rounded-full bg-foreground/[0.07]">
            <div className="h-full rounded-full bg-primary/60" style={{ width: `${Math.round((c.score / top) * 100)}%` }} />
          </div>
          {c.terms && c.terms.length > 0 && (
            <p className="col-span-3 truncate font-mono text-[10.5px] tracking-normal text-muted-foreground/60">{c.terms.join(" · ")}</p>
          )}
        </li>
      ))}
    </ul>
  );
}

function ActionItem({ a }: { a: ActionRecord }) {
  const hasDetails = !!(a.args && Object.keys(a.args).length) || !!a.result || !!a.error;
  return (
    <div className={cn("rounded-[10px] border px-3 py-2", a.error ? "border-destructive/30 bg-destructive/[0.04]" : "border-border")}>
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-mono text-[12px] tracking-normal">{a.name}</span>
        <ModeBadge mode={a.mode} />
        {typeof a.ms === "number" && a.ms > 0 && <span className="text-[10.5px] tabular-nums text-muted-foreground/60">{fmtMs(a.ms)}</span>}
      </div>
      {a.note && <p className="mt-1 text-[11.5px] text-muted-foreground">{a.note}</p>}
      {a.error && <p className="mt-1 text-[11.5px] text-destructive">{a.error}</p>}
      {hasDetails && (
        <Collapsible title="args / result" size="sm" className="mt-2">
          <JsonView value={{ args: a.args ?? {}, result: a.result ?? null, error: a.error ?? null }} maxHeight="max-h-56" />
        </Collapsible>
      )}
    </div>
  );
}

export function TracePanel({ view, names, thresholds = DEFAULT_THRESHOLDS, compact = false, showReply, className }: TracePanelProps) {
  const t = view.trace;
  const path = view.done ? t.path : (view.provisionalPath ?? t.path);
  const detected = t.language?.detected ?? t.triage?.language;
  const replyLang = t.language?.reply || t.reply?.lang;
  const slots = Object.entries(t.decision?.slots ?? {}).filter(([, v]) => v !== null && v !== undefined && v !== "");
  const stateSlots = Object.entries(t.state?.slots ?? {});
  const actions = t.actions ?? [];
  const retrieval = t.retrieval ?? [];
  const errors = t.errors ?? [];
  const notes = t.policy?.notes ?? [];
  const showReplyText = showReply ?? compact;
  const at = view.at ?? (t.at ? Date.parse(t.at) : undefined);

  const header = (
    <div className="flex flex-wrap items-center gap-1.5">
      <span className="text-[12px] font-medium tabular-nums text-foreground/85">Turn {view.turn}</span>
      <PathBadge path={path} provisional={!view.done} />
      <PolicyBadge action={t.policy?.action} />
      <span className="inline-flex items-center gap-1">
        <LangBadge lang={detected} />
        {replyLang && replyLang !== detected && (
          <>
            <ArrowRight className="size-3 text-muted-foreground/50" />
            <LangBadge lang={replyLang} />
          </>
        )}
      </span>
      {at !== undefined && <span className="ml-auto text-[10.5px] tabular-nums text-muted-foreground/55">{fmtTime(at)}</span>}
      {!view.done && !view.error && <span className="size-1.5 animate-pulse rounded-full bg-primary" title="live" />}
    </div>
  );

  const details = (
    <>
      <Section label="signals">
        <SignalChips trace={t} />
      </Section>

      <Section label="path">
        <div className="flex flex-wrap items-center gap-2 text-[12px]">
          <PathBadge path={path} provisional={!view.done} />
          {t.fast_path ? (
            <span className="text-muted-foreground">
              fast path {t.fast_path.eligible ? <span className="text-success">eligible</span> : "not taken"} — {t.fast_path.reason}
              {t.fast_path.candidate && (
                <>
                  {" "}
                  · <span className="font-mono tracking-normal">{t.fast_path.candidate}</span> score {t.fast_path.score?.toFixed(2)} margin {t.fast_path.margin?.toFixed(2)}
                </>
              )}
            </span>
          ) : (
            <span className="text-muted-foreground/45">fast-path check pending</span>
          )}
        </div>
      </Section>

      <Section label="policy">
        {t.policy ? (
          <div className="space-y-1 text-[12px]">
            <div className="flex items-center gap-2">
              <PolicyBadge action={t.policy.action} />
              <ConfidenceBar value={t.policy.confidence} thresholds={thresholds} width="w-14" />
            </div>
            {notes.length > 0 && (
              <ul className="list-disc space-y-0.5 pl-4 text-[11.5px] text-muted-foreground">
                {notes.map((n, i) => (
                  <li key={i}>{n}</li>
                ))}
              </ul>
            )}
          </div>
        ) : (
          <Empty>no verdict yet</Empty>
        )}
      </Section>

      {(retrieval.length > 0 || !view.done) && (
        <Collapsible title="retrieval candidates" count={retrieval.length} defaultOpen={!compact && retrieval.length > 0 && retrieval.length <= 3}>
          {retrieval.length > 0 ? <RetrievalList items={retrieval} names={names} /> : <Empty>no candidates yet</Empty>}
        </Collapsible>
      )}

      <Section label="slots">
        <KV items={(slots.length ? slots : stateSlots).map(([k, v]) => [k, valueText(v)])} />
      </Section>

      <Section label="actions" right={actions.length > 0 ? <span className="text-[10.5px] tabular-nums text-muted-foreground/55">{actions.length}</span> : undefined}>
        {actions.length > 0 ? (
          <div className="space-y-1.5">
            {actions.map((a, i) => (
              <ActionItem key={`${a.name}-${i}`} a={a} />
            ))}
          </div>
        ) : (
          <Empty>no backend actions</Empty>
        )}
      </Section>

      {t.decision?.handoff && t.decision.handoff.queue && (
        <Section label="handoff">
          <div className="rounded-[10px] border border-info/30 bg-info/[0.06] px-3 py-2 text-[12px]">
            <span className="text-info">queue</span> <span className="font-mono tracking-normal">{t.decision.handoff.queue}</span>
            {t.decision.handoff.summary && <p className="mt-1 text-muted-foreground">{t.decision.handoff.summary}</p>}
          </div>
        </Section>
      )}

      <Section label="state">
        {t.state ? (
          <KV
            mono={false}
            items={[
              ["client", t.state.client_id ? `${t.state.client_name ?? "identified"} (${t.state.client_id})` : "not identified"],
              ["active scenario", t.state.active_scenario ? `${t.state.active_scenario} — ${names[t.state.active_scenario] ?? ""}` : "—"],
              ["stack", t.state.stack && t.state.stack.length ? t.state.stack.join(" → ") : "—"],
              [
                "pending confirmation",
                t.state.pending_confirmation ? `${t.state.pending_confirmation.name} (${t.state.pending_confirmation.scenario_id})` : "—",
              ],
              ["language", t.state.language ? langLabel(t.state.language) : "—"],
              ["clarify count", String(t.state.clarify_count ?? 0)],
              ["closed", t.state.closed ? "yes" : "no"],
            ]}
          />
        ) : (
          <Empty>state arrives with turn_done</Empty>
        )}
      </Section>

      <Section label="timings">
        <TimingsWaterfall timings={t.timings} clientE2E={view.client?.e2e} compact={compact} />
      </Section>

      {(t.llm || view.llmStream) && (
        <Section label="llm">
          <div className="space-y-1.5 text-[12px]">
            {t.llm && (
              <p className="text-muted-foreground">
                <span className="text-foreground/85">{t.llm.provider || "—"}</span> · <span className="font-mono tracking-normal">{t.llm.model || "—"}</span>
                {t.llm.ttft_ms > 0 && <> · ttft {fmtMs(t.llm.ttft_ms)}</>}
                {t.llm.total_ms > 0 && <> · total {fmtMs(t.llm.total_ms)}</>}
                {t.llm.usage && typeof t.llm.usage.total_tokens === "number" && (
                  <>
                    {" "}
                    · {t.llm.usage.prompt_tokens ?? "?"}+{t.llm.usage.completion_tokens ?? "?"} tokens
                  </>
                )}
                {t.llm.repaired && <span className="text-warning"> · JSON repaired</span>}
                {t.llm.error && <span className="text-destructive"> · {t.llm.error}</span>}
              </p>
            )}
            {(view.llmStream || t.llm?.raw) && (
              <Collapsible title="raw model output" size="sm" defaultOpen={!view.done && !!view.llmStream}>
                <pre className="max-h-56 overflow-auto whitespace-pre-wrap break-words font-mono text-[11px] leading-[1.5] tracking-normal text-foreground/75">
                  {view.llmStream || t.llm?.raw}
                  {!view.done && view.llmStream && <span className="caret" />}
                </pre>
              </Collapsible>
            )}
            {t.llm?.messages && t.llm.messages.length > 0 && (
              <Collapsible title="messages" count={t.llm.messages.length} size="sm">
                <JsonView value={t.llm.messages} maxHeight="max-h-72" />
              </Collapsible>
            )}
          </div>
        </Section>
      )}

      {t.shadow && (
        <Section label="fast-path shadow check">
          <div className="flex flex-wrap items-center gap-2 text-[12px]">
            {t.shadow.error ? (
              <span className="text-destructive">{t.shadow.error}</span>
            ) : (
              <>
                <Pill tone={t.shadow.agree ? "success" : "warning"}>
                  {t.shadow.agree ? <Check className="size-3" /> : <X className="size-3" />}
                  {t.shadow.agree ? "LLM agrees" : "LLM disagrees"}
                </Pill>
                <span className="text-muted-foreground">
                  LLM primary <span className="font-mono tracking-normal text-foreground/85">{t.shadow.llm_primary || "—"}</span> ({fmtPct(t.shadow.confidence)}) · {fmtMs(t.shadow.ms)}
                </span>
              </>
            )}
          </div>
        </Section>
      )}

      {(errors.length > 0 || view.error) && (
        <Section label="errors">
          <ul className="space-y-1 text-[12px] text-destructive">
            {view.error && (
              <li className="flex items-start gap-1.5">
                <AlertCircle className="mt-0.5 size-3.5 shrink-0" /> {view.error}
              </li>
            )}
            {errors.map((e, i) => (
              <li key={i} className="flex items-start gap-1.5">
                <AlertCircle className="mt-0.5 size-3.5 shrink-0" /> {e}
              </li>
            ))}
          </ul>
        </Section>
      )}
    </>
  );

  return (
    <div className={cn("space-y-4", className)}>
      {header}

      <Section label="transcript">
        <div className="rounded-[12px] border border-border bg-foreground/[0.03] px-3 py-2.5">
          <p className="text-[13px] leading-snug text-foreground/90">{view.transcript || (view.done ? "—" : "…")}</p>
          <div className="mt-1.5 flex flex-wrap items-center gap-1.5 text-[10.5px] text-muted-foreground/60">
            {(view.source || t.input?.source) && <Pill>{view.source || t.input?.source}</Pill>}
            {t.input?.stt_provider && (
              <span>
                stt {t.input.stt_provider}
                {t.input.stt_model ? ` / ${t.input.stt_model}` : ""}
              </span>
            )}
            {typeof t.input?.audio_ms === "number" && t.input.audio_ms > 0 && <span>audio {fmtMs(t.input.audio_ms)}</span>}
          </div>
        </div>
      </Section>

      {showReplyText && (
        <Section label="reply">
          <p className={cn("text-[13px] leading-snug text-foreground/90", !view.replyDone && !view.done && "caret")}>{view.reply || (view.done ? "—" : "")}</p>
        </Section>
      )}

      <Section label="scenario">
        <ScenarioCards trace={t} names={names} thresholds={thresholds} done={view.done} />
      </Section>

      {compact ? (
        <Collapsible title="details" size="sm">
          <div className="space-y-4 pt-1">{details}</div>
        </Collapsible>
      ) : (
        details
      )}
    </div>
  );
}
