import { CtaButton } from "./cta-button";
import { PlusDitherPanel } from "./visuals";

/** Final block: dithered blue marble (Speko-style), a paper card in the middle. */
export function Cta() {
  return (
    <section className="relative z-10 mx-auto w-full max-w-[1200px] px-5 pb-20 sm:px-8 lg:pb-28">
      <div className="relative h-[560px] overflow-hidden rounded-3xl border border-border bg-white">
        <PlusDitherPanel imageSrc="/landing/marble.svg" className="!h-full !w-full" />
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center px-5">
          <div className="pointer-events-auto w-full max-w-[560px] rounded-2xl border border-border bg-white/92 p-8 text-center shadow-[0_30px_80px_-40px_rgba(14,21,18,0.3)] backdrop-blur-sm sm:p-10">
            <h2 className="text-3xl font-medium tracking-[-0.025em] text-ink sm:text-[40px] sm:leading-[1.08]">Скажите роботу, что случилось.</h2>
            <p className="mx-auto mt-4 max-w-[38ch] text-[15px] leading-relaxed text-body">Работает в браузере, без ключей. Попробуйте по-русски, по-казахски или вперемешку.</p>
            <div className="mt-7 flex flex-wrap items-center justify-center gap-3">
              <CtaButton href="/call">Открыть симулятор</CtaButton>
              <CtaButton href="/admin" tone="paper" className="!border !border-border">Консоль</CtaButton>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
