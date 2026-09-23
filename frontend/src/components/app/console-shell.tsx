"use client";
/** Platform top bar: brand, two sections, runtime controls. */
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { resetConversation, setMode, setSttLang, setSttProvider, setTts, useConversation } from "@/lib/store";
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
  const s = useConversation();

  return (
    <>
      <header className="sticky top-0 z-30 border-b border-border bg-background">
        <div className="mx-auto flex h-14 w-full max-w-[1200px] items-center gap-5 px-4 sm:px-8">
          <Link href="/" className="flex shrink-0 items-center gap-2 rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <span aria-hidden className="block size-2.5 rounded-[2px] bg-brand" />
            <span className="text-[15px] font-medium">Bagyt</span>
          </Link>
          <nav aria-label="Разделы" className="flex items-center gap-1">
            {NAV.map((n) => {
              const active = pathname === n.href || pathname.startsWith(n.href + "/");
              return (
                <Link key={n.href} href={n.href} aria-current={active ? "page" : undefined} className={cn("rounded-md px-2.5 py-1 text-sm transition-colors", active ? "bg-muted text-foreground" : "text-muted-foreground hover:text-foreground")}>
                  {n.label}
                </Link>
              );
            })}
          </nav>
          <div className="ml-auto flex items-center gap-3 sm:gap-4">
            <Seg label="Данные" value={s.mode} options={[{ value: "mock", label: "Мок" }, { value: "real", label: "Бэкенд" }] as const} onChange={(m) => setMode(m)} />
            <Seg label="Язык речи" value={s.sttLang} options={[{ value: "ru-RU", label: "RU" }, { value: "kk-KZ", label: "KK" }] as const} onChange={(l) => setSttLang(l)} />
            {s.mode === "mock" && <Seg label="Распознавание" value={s.sttProvider} options={[{ value: "browser", label: "Chrome" }, { value: "server", label: "Сервер" }] as const} onChange={(p) => setSttProvider(p)} />}
            <label className="hidden cursor-pointer items-center gap-2 md:flex">
              <span className="text-xs text-muted-foreground">Озвучка</span>
              <Switch checked={s.tts} onCheckedChange={(v) => setTts(v)} />
            </label>
            <Button variant="secondary" size="sm" onClick={() => resetConversation()}>Новый диалог</Button>
          </div>
        </div>
      </header>
      <main className="flex min-h-0 flex-1 flex-col">{children}</main>
    </>
  );
}
