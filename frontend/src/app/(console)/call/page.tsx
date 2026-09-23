"use client";
import { useUiLanguage, translate as t } from "@/lib/ui-language";
/** /call — what the client sees: just the conversation. */
import { ConversationPanel } from "@/components/app/conversation-panel";
import { useConversation } from "@/lib/store";

export default function CallPage() {
  useUiLanguage();
  const s = useConversation();
  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-1 flex-col px-4 py-6 sm:px-8">
      <div className="mb-4 flex items-baseline justify-between">
        <h1 className="text-xl font-medium">{t("Звонок в Saqta Insurance")}</h1>
        <span className="text-xs text-muted-foreground">{s.mode === "mock" ? t("демо без бэкенда") : s.mode === "core" ? t("LLM · демо-действия") : t("бэкенд")}</span>
      </div>
      <div className="flex h-[calc(100dvh-170px)] min-h-[520px] flex-col overflow-hidden rounded-2xl border border-border bg-background">
        <ConversationPanel className="h-full" />
      </div>
      {s.dialog.client_name && <div className="mt-3 text-xs text-muted-foreground">{t("Клиент:")}{s.dialog.client_name}</div>}
    </div>
  );
}
