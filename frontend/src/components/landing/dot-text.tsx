"use client";
import { useRef } from "react";

/** Dot-matrix words: dots drift, and the cursor reveals solid glyphs underneath. */
export function DotText({ children }: { children: string }) {
  const ref = useRef<HTMLSpanElement>(null);
  const move = (e: React.MouseEvent) => {
    const el = ref.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    el.style.setProperty("--mx", `${e.clientX - r.left}px`);
    el.style.setProperty("--my", `${e.clientY - r.top}px`);
  };
  const leave = () => { ref.current?.style.setProperty("--mx", "-9999px"); ref.current?.style.setProperty("--my", "-9999px"); };
  return (
    <span ref={ref} className="dot-wrap whitespace-nowrap" onMouseMove={move} onMouseLeave={leave}>
      <span className="dot-text dot-drift">{children}</span>
      <span aria-hidden className="dot-solid">{children}</span>
    </span>
  );
}
