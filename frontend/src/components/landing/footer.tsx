import Link from "next/link";
import { PlusMark } from "./header";

export function Footer() {
  return (
    <footer className="border-t border-border">
      <div className="mx-auto flex w-full max-w-[1200px] flex-wrap items-center justify-between gap-4 px-5 py-8 text-sm text-body sm:px-8">
        <div className="flex items-center gap-2 text-ink"><span className="grid size-5 place-items-center rounded bg-brand text-white"><PlusMark className="size-3" /></span> Bagyt · Team Plus · HackAlem AI 2026</div>
        <nav className="flex items-center gap-6">
          <Link href="/call" className="hover:text-ink">Симулятор</Link>
          <Link href="/admin" className="hover:text-ink">Консоль</Link>
          <a href="https://github.com/BAITC-Hacks/hack-9dd7b5f3-plus" className="hover:text-ink" target="_blank" rel="noreferrer">GitHub</a>
        </nav>
      </div>
    </footer>
  );
}
