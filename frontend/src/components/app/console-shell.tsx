"use client";
/**
 * Console shell — Speko console top bar (dark-first): brand, segmented nav,
 * runtime controls (mode / STT language / TTS / new dialog) and status badge.
 */
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { segmentedControlItemVariants, segmentedControlRootClassName } from "@/lib/segmented-control";
import { resetConversation, setMode, setSttLang, setTts, useConversation } from "@/lib/store";
import { cn } from "@/lib/utils";

const NAV = [
  { href: "/call", label: "Симулятор" },
  { href: "/admin", label: "Консоль" },
] as const;

function Segmented<T extends string>({
  value,
  options,
  onChange,
  label,
  mono = true,
}: {
  value: T;
  options: readonly { value: T; label: string }[];
  onChange: (v: T) => void;
  label: string;
  mono?: boolean;
}) {
  return (
    <div className="flex items-center gap-2">
      <span className="marker hidden text-muted-foreground lg:inline">{label}</span>
      <div role="radiogroup" aria-label={label} className={cn(segmentedControlRootClassName, "h-7")}>
        {options.map((o) => (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={o.value === value}
            data-checked={o.value === value ? "" : undefined}
            onClick={() => onChange(o.value)}
            className={cn(
              segmentedControlItemVariants({ size: "sm", state: "checked" }),
              "h-6 sm:h-6 sm:text-xs",
              mono && "font-mono text-[11px] uppercase tracking-[.08em] sm:text-[11px]",
            )}
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
      <header className="hairline-b sticky top-0 z-30 bg-background">
        <div className="mx-auto flex h-14 w-full max-w-[1248px] items-center gap-4 px-4 sm:px-8">
          {/* brand */}
          <Link href="/" className="flex shrink-0 items-center gap-2.5 outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm">
            <span aria-hidden className="block size-[11px] bg-brand" />
            <span className="font-heading text-[15px] font-semibold tracking-[-0.022em]">Bagyt</span>
            <span className="marker ml-1 hidden text-muted-foreground xl:inline">VOICE ROUTER · SAQTA INSURANCE</span>
          </Link>

          {/* nav */}
          <nav aria-label="Разделы" className={cn(segmentedControlRootClassName, "h-8 shrink-0")}>
            {NAV.map((n) => {
              const active = pathname === n.href || pathname.startsWith(n.href + "/");
              return (
                <Link
                  key={n.href}
                  href={n.href}
                  aria-current={active ? "page" : undefined}
                  className={cn(segmentedControlItemVariants({ size: "sm", state: "current" }), "h-7 sm:h-7")}
                >
                  {n.label}
                </Link>
              );
            })}
          </nav>

          <div className="ml-auto flex items-center gap-3 sm:gap-4">
            <Segmented
              label="режим"
              value={s.mode}
              options={[{ value: "mock", label: "mock" }, { value: "real", label: "real" }] as const}
              onChange={(m) => setMode(m)}
            />
            <Segmented
              label="stt"
              value={s.sttLang}
              options={[{ value: "ru-RU", label: "RU" }, { value: "kk-KZ", label: "KK" }] as const}
              onChange={(l) => setSttLang(l)}
            />
            <label className="hidden cursor-pointer items-center gap-2 md:flex">
              <span className="marker text-muted-foreground">озвучка</span>
              <Switch checked={s.tts} onCheckedChange={(v) => setTts(v)} className="[--thumb-size:--spacing(3.5)] sm:[--thumb-size:--spacing(3.5)]" />
            </label>
            <Button variant="secondary" size="sm" onClick={() => resetConversation()}>
              Новый диалог
            </Button>
            <Badge variant={s.mode === "mock" ? "warning" : "success"} className="font-mono uppercase tracking-[.08em]">
              {s.mode === "mock" ? "mock" : "backend"}
            </Badge>
          </div>
        </div>
      </header>
      <main className="flex min-h-0 flex-1 flex-col">{children}</main>
    </>
  );
}
