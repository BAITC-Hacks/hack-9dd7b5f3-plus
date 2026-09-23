import Link from "next/link";
import { CtaButton } from "./cta-button";

export function Hero() {
  return (
    <section className="relative z-10 mx-auto w-full max-w-[1200px] px-5 pt-24 text-center sm:px-8 sm:pt-32">
      <h1 className="mx-auto max-w-[16ch] text-[46px] font-medium leading-[1.02] tracking-[-0.035em] text-ink sm:text-[72px] lg:text-[92px]">
        Робот, который понимает <span className="dot-text whitespace-nowrap">с первой фразы</span>
      </h1>
      <p className="mx-auto mt-8 max-w-[44ch] text-lg leading-relaxed text-body sm:text-[21px]">
        Русский, казахский и смешанная речь. Сорок сценариев, выбор с объяснением, ответ за полторы секунды.
      </p>
      <div className="mt-10 flex flex-wrap items-center justify-center gap-4">
        <CtaButton href="/call">Поговорить с роботом</CtaButton>
        <Link href="/admin" className="inline-flex h-12 items-center rounded-full border border-border bg-white px-6 text-sm font-medium text-ink transition-colors hover:border-ink/30">
          Консоль супервизора
        </Link>
      </div>

      {/* blurred blue horizon, stepped like a skyline */}
      <div aria-hidden className="relative mt-16 h-[240px] overflow-hidden sm:mt-20 sm:h-[300px]">
        <div className="absolute inset-x-[-6%] bottom-[-90px] flex h-[400px] items-end blur-xl">
          {[62, 48, 70, 55, 80, 44, 66, 58, 74, 50, 68, 46].map((h, i) => (
            <div key={i} className="flex-1" style={{ height: `${h}%`, background: "linear-gradient(to bottom, rgba(47,106,209,0.08) 0%, #2f6ad1 32%, #2a5ebb 100%)" }} />
          ))}
        </div>
        <div className="absolute inset-x-0 bottom-0 h-8 bg-gradient-to-b from-transparent to-paper/90" />
      </div>
    </section>
  );
}
