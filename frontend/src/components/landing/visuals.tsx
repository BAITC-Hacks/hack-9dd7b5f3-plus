"use client";
/** Client-only wrappers for the WebGL/canvas blocks (they touch window). */
import dynamic from "next/dynamic";

export const PlusDitherPanel = dynamic(() => import("@/components/block/plus-dither").then((m) => m.PlusDither), {
  ssr: false,
  loading: () => <div className="h-full w-full bg-[radial-gradient(circle_at_60%_45%,#bed2ef,transparent_70%)]" />,
});

export const GlassPanel = dynamic(() => import("@/components/block/fractal-glass").then((m) => m.FractalGlass), {
  ssr: false,
  loading: () => <div className="absolute inset-0 bg-[url(/landing/plus.svg)] bg-cover bg-center" />,
});

export const DotsPanel = dynamic(() => import("@/components/block/dotted-grid").then((m) => m.DottedGrid), { ssr: false, loading: () => null });
