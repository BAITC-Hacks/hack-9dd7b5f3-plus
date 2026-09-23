/** One heading rhythm for every section: h2 + one paragraph, left-aligned, same widths. */
export function SectionHead({ title, text }: { title: string; text: string }) {
  return (
    <div className="mb-10 max-w-3xl">
      <h2 className="text-3xl font-medium tracking-[-0.02em] text-ink sm:text-[40px] sm:leading-[1.1]">{title}</h2>
      <p className="mt-4 max-w-[60ch] text-[17px] leading-relaxed text-body">{text}</p>
    </div>
  );
}
export const SECTION = "relative z-10 mx-auto w-full max-w-[1200px] px-5 py-20 sm:px-8 lg:py-24";
export const CARD = "lift rounded-3xl border border-border bg-white";
