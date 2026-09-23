import Link from "next/link";
import { CtaButton } from "./cta-button";
import { PlusDitherPanel } from "./visuals";

export function Hero() {
  return (
    <section className="mx-auto grid w-full max-w-[1200px] items-center gap-12 px-5 pb-20 pt-16 sm:px-8 lg:grid-cols-[minmax(0,7fr)_minmax(0,5fr)] lg:pb-28 lg:pt-24">
      <div>
        <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-border bg-white px-3 py-1 text-xs text-body">
          <span className="size-1.5 rounded-full bg-brand" />
          Voice Router для контакт-центра · кейс Halyk Bank
        </div>
        <h1 className="max-w-[12ch] text-[44px] font-medium leading-[1.02] tracking-[-0.03em] text-ink sm:text-[64px] lg:text-[76px]">
          Робот, который понимает с первой фразы.
        </h1>
        <p className="mt-6 max-w-[52ch] text-lg leading-relaxed text-body">
          Клиент говорит своими словами, по-русски, по-казахски или вперемешку. Bagyt выбирает нужный сценарий из сорока, держит контекст при смене темы и объясняет супервизору каждое решение.
        </p>
        <div className="mt-9 flex flex-wrap items-center gap-5">
          <CtaButton href="/call">Поговорить с роботом</CtaButton>
          <Link href="/admin" className="group inline-flex items-center gap-2 text-sm font-medium text-ink">
            Консоль супервизора
            <span className="transition-transform group-hover:translate-x-0.5">→</span>
          </Link>
        </div>
        <dl className="mt-12 grid max-w-md grid-cols-3 gap-6 border-t border-border pt-6 text-sm">
          <div><dt className="text-body">Сценариев</dt><dd className="mt-1 text-2xl font-medium tabular-nums text-ink">40</dd></div>
          <div><dt className="text-body">Языки</dt><dd className="mt-1 text-2xl font-medium text-ink">ru · kk</dd></div>
          <div><dt className="text-body">До ответа</dt><dd className="mt-1 text-2xl font-medium tabular-nums text-ink">≤ 1,5 с</dd></div>
        </dl>
      </div>
      <div className="relative aspect-[4/5] w-full overflow-hidden rounded-3xl border border-border bg-white sm:aspect-square lg:aspect-[4/5]">
        <PlusDitherPanel className="!h-full !w-full" />
        <div className="pointer-events-none absolute bottom-4 left-4 rounded-full bg-white/85 px-3 py-1 text-xs text-body backdrop-blur">Team Plus · проведите курсором</div>
      </div>
    </section>
  );
}
