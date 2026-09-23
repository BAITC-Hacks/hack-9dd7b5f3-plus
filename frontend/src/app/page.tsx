"use client";

import { useEffect, useMemo, useState, useSyncExternalStore } from "react";
import { CallController } from "@/lib/call-controller";
import { useScenarioNames } from "@/lib/catalog-cache";
import { shortId } from "@/lib/format";
import { CallControls } from "@/components/call/call-controls";
import { Composer } from "@/components/call/composer";
import { Conversation } from "@/components/call/conversation";
import { MicButton } from "@/components/call/mic-button";
import { StatusLine } from "@/components/call/status-line";
import { TurnStrip } from "@/components/call/turn-strip";
import { PageBody, PageHeader } from "@/components/shell/page-header";
import { TracePanel } from "@/components/trace/trace-panel";
import { Panel } from "@/components/ui/panel";
import { Pill } from "@/components/ui/pill";
import { EmptyState } from "@/components/ui/states";
import { Activity } from "lucide-react";

function isTypingTarget(el: EventTarget | null): boolean {
  if (!(el instanceof HTMLElement)) return false;
  const tag = el.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || el.isContentEditable;
}

export default function CallPage() {
  const [ctl] = useState(() => new CallController());
  const st = useSyncExternalStore(ctl.subscribe, ctl.getSnapshot, ctl.getServerSnapshot);
  const names = useScenarioNames();

  useEffect(() => {
    ctl.mount();
    return () => ctl.unmount();
  }, [ctl]);

  // Space = push-to-talk when nothing is focused for typing.
  useEffect(() => {
    if (st.mode !== "ptt") return;
    const down = (e: KeyboardEvent) => {
      if (e.code !== "Space" || e.repeat || isTypingTarget(e.target)) return;
      e.preventDefault();
      void ctl.pttStart();
    };
    const up = (e: KeyboardEvent) => {
      if (e.code !== "Space" || isTypingTarget(e.target)) return;
      e.preventDefault();
      ctl.pttEnd();
    };
    window.addEventListener("keydown", down);
    window.addEventListener("keyup", up);
    return () => {
      window.removeEventListener("keydown", down);
      window.removeEventListener("keyup", up);
    };
  }, [ctl, st.mode]);

  const thresholds = useMemo(
    () => ({ proceed: st.config?.policy.proceed_min ?? 0.55, clarify: st.config?.policy.clarify_min ?? 0.3 }),
    [st.config],
  );

  const selected = useMemo(() => {
    if (st.turns.length === 0) return null;
    if (st.selectedTurn === null) return st.turns[st.turns.length - 1];
    return st.turns.find((t) => t.turn === st.selectedTurn) ?? st.turns[st.turns.length - 1];
  }, [st.turns, st.selectedTurn]);

  const busy = st.wsStatus === "connecting";

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageBody className="flex min-h-0 flex-1 flex-col gap-4 py-4" wide>
        <PageHeader
          title="Call"
          meta={
            <span className="flex items-center gap-1.5">
              {st.sessionId && (
                <Pill mono title={st.sessionId}>
                  {shortId(st.sessionId, 12)}
                </Pill>
              )}
              {st.config?.llm.provider === "mock" && (
                <Pill tone="warning" title="LLM_PROVIDER=mock: the keyless lexical router answers">
                  keyless mode
                </Pill>
              )}
              {st.sessionState?.active_scenario && (
                <Pill tone="primary" mono title="active scenario">
                  {st.sessionState.active_scenario}
                </Pill>
              )}
              {st.sessionState?.client_name && <Pill tone="info">{st.sessionState.client_name}</Pill>}
            </span>
          }
        />

        <div className="grid min-h-0 flex-1 gap-4 lg:grid-cols-[minmax(0,1fr)_400px] xl:grid-cols-[minmax(0,1fr)_460px]">
          <section className="flex min-h-[520px] flex-col rounded-[15px] border border-border bg-card lg:min-h-0">
            <Conversation
              turns={st.turns}
              partial={st.partial}
              micState={st.micState}
              selectedTurn={st.selectedTurn}
              onSelect={(t) => ctl.selectTurn(t)}
              onSend={(text) => void ctl.sendText(text)}
              className="min-h-0 flex-1"
            />
            <div className="space-y-3 border-t border-border px-4 pb-3 pt-3">
              <div className="flex flex-col items-center gap-3 sm:flex-row sm:items-end sm:justify-between">
                <div className="order-2 w-full min-w-0 sm:order-1 sm:w-auto sm:flex-1">
                  <CallControls
                    st={st}
                    onMode={(m) => ctl.setMode(m)}
                    onStt={(s) => ctl.setSttSource(s)}
                    onLang={(l) => ctl.setBrowserLang(l)}
                    onMuted={(m) => ctl.setMuted(m)}
                    onBargeIn={(b) => ctl.setBargeIn(b)}
                    onSensitivity={(v) => ctl.setSensitivity(v)}
                    onNewSession={() => void ctl.newSession()}
                  />
                </div>
                <div className="order-1 shrink-0 sm:order-2">
                  <MicButton
                    state={st.micState}
                    armed={st.armed}
                    inSpeech={st.inSpeech}
                    mode={st.mode}
                    disabled={busy || (st.wsStatus !== "open" && st.wsStatus !== "idle")}
                    subscribeLevel={ctl.subscribeLevel}
                    getLevel={ctl.getLevel}
                    onToggle={() => void ctl.toggleArmed()}
                    onPttStart={() => void ctl.pttStart()}
                    onPttEnd={() => ctl.pttEnd()}
                    onStop={() => ctl.cancel()}
                  />
                </div>
              </div>
              <Composer onSend={(text) => void ctl.sendText(text)} disabled={busy} />
            </div>
            <StatusLine st={st} onDismiss={() => ctl.dismissStatus()} />
          </section>

          <aside className="flex min-h-0 flex-col gap-3 lg:overflow-y-auto lg:pr-0.5">
            <TurnStrip turns={st.turns} selectedTurn={st.selectedTurn} onSelect={(t) => ctl.selectTurn(t)} />
            <Panel title="Trace" note={selected ? `turn ${selected.turn} of ${st.turns.length}` : undefined} className="min-h-[320px]">
              {selected ? (
                <TracePanel view={selected} names={names} thresholds={thresholds} />
              ) : (
                <EmptyState
                  icon={<Activity className="size-4" />}
                  title="No turns yet"
                  hint="The trace shows the transcript, chosen scenario, signals, policy verdict, actions and a timings waterfall for every turn — live, as events arrive."
                />
              )}
            </Panel>
          </aside>
        </div>
      </PageBody>
    </div>
  );
}
