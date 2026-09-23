import Link from "next/link";
import { CtaButton } from "./cta-button";
import { DotText } from "./dot-text";

export function Hero() {
  return (
    <section className="relative z-10 mx-auto w-full max-w-[1200px] px-5 pt-24 text-center sm:px-8 sm:pt-32">
      <h1 className="mx-auto max-w-[24ch] text-[46px] font-medium leading-[1.02] tracking-[-0.035em] text-ink sm:text-[72px] lg:text-[88px]">
        Робот, который понимает<br className="hidden sm:block" /> <DotText>с первой фразы</DotText>
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

    </section>
  );
}

/** Blurred blue horizon, stepped like a skyline; full-bleed, no hard edges, flows into the first band. */
export function Horizon() {
  return (
    <div aria-hidden className="relative z-10 mt-10 h-[300px] overflow-hidden sm:mt-12 sm:h-[340px]" style={{ maskImage: "linear-gradient(to bottom, transparent 0%, #000 34%)", WebkitMaskImage: "linear-gradient(to bottom, transparent 0%, #000 34%)" }}>
      <div className="absolute inset-x-[-4%] bottom-[-70px] flex h-[300px] items-end blur-2xl">
        {[58, 44, 68, 52, 78, 40, 64, 56, 72, 48, 66, 42, 60, 50].map((h, i) => (
          <div key={i} className="sky-col flex-1" style={{ animationDelay: `${i * 0.45}s`, height: `${h}%`, background: "linear-gradient(to bottom, rgba(47,106,209,0) 0%, #2f6ad1 38%, #2a5ebb 100%)" }} />
        ))}
      </div>
      <div className="absolute inset-x-0 bottom-0 h-16" style={{ background: "linear-gradient(to bottom, rgba(42,94,187,0), #2a5ebb 75%)" }} />
    </div>
  );
}
