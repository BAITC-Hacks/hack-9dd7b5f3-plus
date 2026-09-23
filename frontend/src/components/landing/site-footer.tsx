import Link from "next/link";
import { BrandMark, Container } from "./section";

export function SiteFooter() {
  return (
    <footer className="hairline-t mt-8 py-10">
      <Container className="flex flex-col gap-6 md:flex-row md:items-center md:justify-between">
        <div className="flex flex-col gap-3">
          <BrandMark />
          <span className="marker">Team Plus · HackAlem AI · 2026</span>
        </div>
        <nav className="marker flex flex-wrap gap-x-6 gap-y-2">
          <Link href="/call" className="hover:text-ink">
            Симулятор
          </Link>
          <Link href="/admin" className="hover:text-ink">
            Консоль
          </Link>
          <a href="https://github.com/BAITC-Hacks/hack-9dd7b5f3-plus" target="_blank" rel="noreferrer" className="hover:text-ink">
            Репозиторий
          </a>
        </nav>
      </Container>
    </footer>
  );
}
