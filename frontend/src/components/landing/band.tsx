import { PixelCluster, PixelField } from "./clouds";

/** Speko-style section band: big noun left with a pixel cluster, one-sentence description right. */
export function Band({ id, title, text }: { id?: string; title: string; text: string }) {
  return (
    <div id={id} className="relative z-10 scroll-mt-24 border-y border-border bg-paper-2/70">
      <PixelField />
      <div className="relative mx-auto grid w-full max-w-[1200px] items-center gap-4 px-5 py-9 sm:px-8 md:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] md:gap-10">
        <div className="flex items-center gap-4 text-3xl font-medium tracking-[-0.02em] text-ink sm:text-4xl">
          {title}
          <PixelCluster className="hidden sm:inline-block" />
        </div>
        <p className="text-[17px] leading-relaxed text-body md:text-right">{text}</p>
      </div>
    </div>
  );
}
