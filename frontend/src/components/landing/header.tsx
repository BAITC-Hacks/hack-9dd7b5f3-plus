import Link from "next/link";
import { CtaButton } from "./cta-button";

export function Header() {
  return (
    <div className="sticky top-4 z-40 px-4 sm:px-8">
      <header className="mx-auto flex h-14 w-full max-w-[1200px] items-center justify-between rounded-2xl border border-border bg-white/85 px-4 shadow-[0_1px_0_rgba(14,21,18,0.04)] backdrop-blur sm:px-5">
        <Link href="/" className="flex items-center gap-2.5">
          <span aria-hidden className="grid size-7 place-items-center rounded-lg bg-brand text-white"><PlusMark /></span>
          <span className="text-[15px] font-medium tracking-tight text-ink">Bagyt</span>
        </Link>
        <nav className="hidden items-center gap-8 text-[15px] text-body md:flex">
          <a href="#console" className="hover:text-ink">Консоль</a>
          <a href="#scenarios" className="hover:text-ink">Сценарии</a>
          <a href="#trace" className="hover:text-ink">Трассировка</a>
          <Link href="/admin" className="hover:text-ink">Открыть консоль</Link>
        </nav>
        <CtaButton href="/call" className="!h-10">Открыть симулятор</CtaButton>
      </header>
    </div>
  );
}

export function PlusMark({ className = "size-3.5" }: { className?: string }) {
  return (
    <svg viewBox="0 0 20 20" className={className} fill="currentColor" aria-hidden>
      <rect x="8" y="2" width="4" height="16" rx="1" />
      <rect x="2" y="8" width="16" height="4" rx="1" />
    </svg>
  );
}
