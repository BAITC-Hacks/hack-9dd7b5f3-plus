/** Dithered blue clouds on the page edges — Speko's signature, done with a dot mask over a gradient. */
export function EdgeClouds() {
  return (
    <>
      <div aria-hidden className="dither-cloud pointer-events-none fixed inset-y-0 left-0 z-0 hidden w-[26vw] lg:block" style={{ background: "radial-gradient(70% 55% at -10% 62%, #2f6ad1 0%, #6f8fd8 30%, #bed2ef 55%, transparent 75%)" }} />
      <div aria-hidden className="dither-cloud pointer-events-none fixed inset-y-0 right-0 z-0 hidden w-[26vw] lg:block" style={{ background: "radial-gradient(70% 55% at 110% 40%, #2f6ad1 0%, #6f8fd8 30%, #bed2ef 55%, transparent 75%)" }} />
    </>
  );
}

/** Scattered pixel cluster (decoration next to band titles). Deterministic positions. */
const PIX: Array<[number, number, number, number]> = [
  [0, 2, 6, 0.9], [8, 0, 4, 0.5], [14, 6, 6, 0.8], [22, 3, 4, 0.4], [30, 8, 6, 0.9], [36, 1, 4, 0.6], [4, 12, 4, 0.5], [12, 14, 6, 0.7], [20, 11, 4, 0.9], [28, 16, 4, 0.5], [40, 12, 6, 0.6], [46, 6, 4, 0.8], [52, 2, 6, 0.5], [50, 14, 4, 0.9], [60, 9, 4, 0.6], [66, 4, 6, 0.7], [72, 12, 4, 0.5], [78, 7, 4, 0.9], [84, 2, 6, 0.4], [90, 10, 4, 0.7], [96, 5, 4, 0.8], [104, 13, 6, 0.5], [110, 3, 4, 0.9], [118, 8, 4, 0.6],
];
export function PixelCluster({ className = "" }: { className?: string }) {
  return (
    <span aria-hidden className={"relative inline-block h-6 w-32 align-middle " + className}>
      {PIX.map(([x, y, s, o], i) => (
        <span key={i} className="absolute rounded-[1px] bg-brand" style={{ left: x, top: y, width: s, height: s, opacity: o }} />
      ))}
    </span>
  );
}
