"use client";
/**
 * /call — the CLIENT's view of the voice robot. Minimal, no router internals
 * except a thin muted status strip under the conversation card.
 */
import { ConversationPanel } from "@/components/app/conversation-panel";
import { Card } from "@/components/ui/card";
import { Kbd } from "@/components/ui/kbd";
import { scenarioLabel } from "@/lib/catalog";
import { useConversation } from "@/lib/store";

export default function CallPage() {
  const s = useConversation();
  const last = s.traces.length ? s.traces[s.traces.length - 1] : null;
  const lastMs = last?.latency_ms.total;
  const scenario = s.dialog.active_scenario ? scenarioLabel(s.dialog.active_scenario, "ru") : "—";

  return (
    <div className="mx-auto flex w-full max-w-[1248px] flex-1 flex-col px-4 py-6 sm:px-8">
      <div className="mb-4 flex flex-wrap items-end justify-between gap-2">
        <h1 className="h-display text-2xl">Симулятор звонка</h1>
        <div className="marker text-muted-foreground">клиент · голос или текст · {s.mode}</div>
      </div>

      <Card className="flex h-[calc(100dvh-180px)] min-h-[560px] flex-col overflow-hidden rounded-xl p-0 shadow-none">
        <ConversationPanel className="h-full" />
      </Card>

      <div className="mt-3 flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <div className="marker text-muted-foreground">
          клиент: <span className="text-foreground/80">{s.dialog.client_name ?? "не идентифицирован"}</span>
          {" · "}язык ответа: <span className="text-foreground/80">{s.dialog.language}</span>
          {" · "}активный сценарий: <span className="text-foreground/80">{scenario}</span>
          {" · "}последний ход: <span className="num text-foreground/80">{lastMs != null ? `${Math.round(lastMs)} ms` : "—"}</span>
        </div>
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Kbd>Зажмите микрофон</Kbd>
          <span>отпустите — отправится</span>
        </div>
      </div>
    </div>
  );
}
