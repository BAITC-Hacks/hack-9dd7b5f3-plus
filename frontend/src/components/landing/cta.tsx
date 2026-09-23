import { CtaButton } from "./cta-button";
import { DotsPanel } from "./visuals";

export function Cta() {
  return (
    <section className="mx-auto w-full max-w-[1200px] px-5 pb-20 sm:px-8 lg:pb-28">
      <div className="relative overflow-hidden rounded-3xl bg-[#0d0d0d] px-6 py-20 text-center text-white sm:px-12">
        <div className="absolute inset-0 opacity-40"><DotsPanel className="!h-full !w-full" /></div>
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_center,rgba(13,13,13,0.92)_0%,rgba(13,13,13,0.7)_38%,transparent_70%)]" />
        <div className="relative">
          <h2 className="mx-auto max-w-[18ch] text-3xl font-medium tracking-[-0.02em] sm:text-5xl">Скажите роботу, что случилось.</h2>
          <p className="mx-auto mt-4 max-w-[46ch] text-base text-white/70">Работает в браузере, без ключей. Русский, казахский, смешанная речь.</p>
          <div className="mt-8 flex justify-center"><CtaButton href="/call" tone="paper">Открыть симулятор</CtaButton></div>
        </div>
      </div>
    </section>
  );
}
