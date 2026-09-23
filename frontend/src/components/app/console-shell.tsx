"use client";
/** Platform top bar: brand · Звонок | Аналитика · language · engine · new dialog. Nothing else. */
import { RotateCcw } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect } from "react";
import { EnterReveal } from "@/components/app/enter-reveal";
import { LogoMark, Wordmark } from "@/components/brand";
import { resetConversation, useConversation } from "@/lib/store";
import { restoreUiLanguage, setUiLanguage, useUiLanguage, translate as t } from "@/lib/ui-language";
import { cn } from "@/lib/utils";

const NAV = [
  { href: "/call", label: "Звонок" },
  { href: "/admin", label: "Аналитика" },
] as const;

export function ConsoleShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const language = useUiLanguage();
  const s = useConversation();
  useEffect(() => { restoreUiLanguage(); }, []);
  const busy = s.status === "thinking" || s.status === "listening";

  return (
    <>
      <EnterReveal />
      <header className="platform-header sticky top-0 z-30 border-b border-border bg-background/90 backdrop-blur">
        <div className="mx-auto grid h-14 w-full max-w-[1200px] grid-cols-[1fr_auto_1fr] items-center px-4 sm:px-8">
          <Link href="/" className="flex w-fit items-center gap-2 rounded-md text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <LogoMark className="size-6" dark />
            <Wordmark className="text-[15px]" />
          </Link>

          <nav aria-label={t("Разделы")} className="flex h-9 items-center gap-0.5 rounded-full border border-border bg-card p-0.5">
            {NAV.map((n) => {
              const active = pathname === n.href || pathname.startsWith(n.href + "/");
              return (
                <Link
                  key={n.href}
                  href={n.href}
                  aria-current={active ? "page" : undefined}
                  className={cn("flex h-full items-center rounded-full px-4 text-sm transition-colors", active ? "bg-foreground text-background" : "text-muted-foreground hover:text-foreground")}
                >
                  {t(n.label)}
                </Link>
              );
            })}
          </nav>

          <div className="flex items-center justify-end gap-2 sm:gap-3">
            <span className="hidden items-center gap-1.5 text-xs text-muted-foreground md:inline-flex" title={s.mode === "mock" ? "Демо без ключей" : "LLM-маршрутизатор"}>
              <span className={cn("size-1.5 rounded-full", s.mode === "mock" ? "bg-warning" : "bg-success")} />
              {s.mode === "mock" ? t("демо") : "LLM"}
            </span>
            <div role="radiogroup" aria-label={t("Язык")} className="flex h-8 items-center rounded-full border border-border bg-card p-0.5">
              {(["ru", "kk"] as const).map((l) => (
                <button
                  key={l}
                  type="button"
                  role="radio"
                  aria-checked={language === l}
                  onClick={() => setUiLanguage(l)}
                  className={cn("h-full rounded-full px-2.5 text-xs font-medium uppercase transition-colors", language === l ? "bg-foreground text-background" : "text-muted-foreground hover:text-foreground")}
                >
                  {l === "ru" ? "RU" : "KK"}
                </button>
              ))}
            </div>
            <button
              type="button"
              onClick={() => resetConversation()}
              disabled={busy || s.messages.length === 0}
              className="inline-flex h-8 items-center gap-1.5 rounded-full px-3 text-xs text-muted-foreground transition-colors hover:bg-card hover:text-foreground disabled:opacity-40"
            >
              <RotateCcw className="size-3.5" />
              <span className="hidden sm:inline">{t("Новый диалог")}</span>
            </button>
          </div>
        </div>
      </header>
      <main className="platform-main flex min-h-0 flex-1 flex-col">{children}</main>
    </>
  );
}
