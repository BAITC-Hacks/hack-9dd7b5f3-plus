"use client";
/** Platform top bar: brand, two sections, runtime controls. */
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect } from "react";
import { EnterReveal } from "@/components/app/enter-reveal";
import { restoreUiLanguage, setUiLanguage, useUiLanguage, translate as t } from "@/lib/ui-language";
import { cn } from "@/lib/utils";

const NAV = [
  { href: "/call", label: "Звонок" },
  { href: "/admin", label: "Консоль" },
] as const;

function Seg<T extends string>({ value, options, onChange, label }: { value: T; options: readonly { value: T; label: string }[]; onChange: (v: T) => void; label: string }) {
  return (
    <div className="flex items-center gap-2">
      <span className="hidden text-xs text-muted-foreground lg:inline">{label}</span>
      <div role="radiogroup" aria-label={label} className="flex h-7 items-center rounded-lg bg-muted p-0.5">
        {options.map((o) => (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={o.value === value}
            onClick={() => onChange(o.value)}
            className={cn("h-6 rounded-md px-2.5 text-xs transition-colors", o.value === value ? "bg-background text-foreground" : "text-muted-foreground hover:text-foreground")}
          >
            {o.label}
          </button>
        ))}
      </div>
    </div>
  );
}

export function ConsoleShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const language = useUiLanguage();
  useEffect(() => { restoreUiLanguage(); }, []);

  return (
    <>
      <EnterReveal />
      <header className="sticky top-0 z-30 border-b border-border bg-background">
        <div className="mx-auto flex h-14 w-full max-w-[1200px] items-center gap-5 px-4 sm:px-8">
          <nav aria-label={t("Разделы")} className="flex items-center gap-1">
            {NAV.map((n) => {
              const active = pathname === n.href || pathname.startsWith(n.href + "/");
              return (
                <Link key={n.href} href={n.href} aria-current={active ? "page" : undefined} className={cn("rounded-md px-2.5 py-1 text-sm transition-colors", active ? "bg-muted text-foreground" : "text-muted-foreground hover:text-foreground")}>
                  {t(n.label)}
                </Link>
              );
            })}
          </nav>
          <div className="ml-auto flex items-center gap-3 sm:gap-4">
            <Seg label={t("Язык интерфейса")} value={language} options={[{ value: "ru", label: "RU" }, { value: "kk", label: "ҚАЗ" }] as const} onChange={setUiLanguage} />
          </div>
        </div>
      </header>
      <main className="flex min-h-0 flex-1 flex-col">{children}</main>
    </>
  );
}
