import { cn } from "@/lib/utils";

/** Landing grid: 1248px max width, 32px gutter (16px on mobile). */
export function Container({ className, children }: { className?: string; children: React.ReactNode }) {
  return <div className={cn("mx-auto w-full max-w-[1248px] px-4 md:px-8", className)}>{children}</div>;
}

/** Section header: "[ 03 / 05 ]  МАРШРУТИЗАЦИЯ" + noun title, no description line. */
export function SectionHead({ index, marker, title, meta }: { index: string; marker: string; title: string; meta?: string }) {
  return (
    <header className="mb-8 flex flex-col gap-3">
      <span className="marker marker-dot">
        [ {index} / 05 ]&nbsp;&nbsp;{marker}
      </span>
      <h2 className="h-display text-ink text-3xl md:text-[2.5rem]">{title}</h2>
      {meta ? <span className="marker">{meta}</span> : null}
    </header>
  );
}

export function BrandMark({ className }: { className?: string }) {
  return (
    <span className={cn("inline-flex items-center gap-2.5", className)}>
      <span aria-hidden className="inline-block size-3 shrink-0 bg-brand" />
      <span className="font-heading text-ink text-[17px] font-semibold tracking-[-0.022em]">Bagyt</span>
    </span>
  );
}
