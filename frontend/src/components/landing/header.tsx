import Link from "next/link";
import { CtaButton } from "./cta-button";

export function Header() {
  return (
    <header className="sticky top-0 z-40 border-b border-border/60 bg-paper/80 backdrop-blur">
      <div className="mx-auto flex h-16 w-full max-w-[1200px] items-center justify-between px-5 sm:px-8">
        <Link href="/" className="flex items-center gap-2.5">
          <span aria-hidden className="grid size-6 place-items-center rounded-md bg-brand text-white"><PlusMark /></span>
          <span className="text-[15px] font-medium tracking-tight">Bagyt</span>
        </Link>
        <nav className="hidden items-center gap-7 text-sm text-body md:flex">
          <a href="#product" className="hover:text-ink">Продукт</a>
          <a href="#how" className="hover:text-ink">Как работает</a>
          <a href="#why" className="hover:text-ink">Почему LLM</a>
          <Link href="/admin" className="hover:text-ink">Консоль</Link>
        </nav>
        <CtaButton href="/call" className="!h-10">Открыть симулятор</CtaButton>
      </div>
    </header>
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
