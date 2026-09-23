"use client";
import { useEffect } from "react";
import { buildWaveStack } from "./page-waves";

/**
 * When the page was opened through a CTA wave, the sheets are already covering (black on top).
 * Blue and white slip away underneath, then the black sheet lifts slowly with a soft ease-out,
 * so the dark platform appears seamlessly.
 */
export function EnterReveal() {
  useEffect(() => {
    let flagged = false;
    try { flagged = sessionStorage.getItem("bagyt:enter") === "1"; if (flagged) sessionStorage.removeItem("bagyt:enter"); } catch { /* noop */ }
    document.querySelectorAll(".wave-stack").forEach((el) => el.remove());
    if (!flagged) return;
    document.body.dataset.enter = "1";
    const clear = window.setTimeout(() => { delete document.body.dataset.enter; }, 1900);
    const stack = buildWaveStack("covering");
    const waves = [...stack.querySelectorAll<HTMLElement>(".wave")]; // blue, white, black
    waves.forEach((w, i) => {
      const isBlack = i === waves.length - 1;
      w.style.transitionDelay = isBlack ? "240ms" : `${i * 90}ms`;
      w.style.transitionDuration = isBlack ? "1.25s" : "0.8s";
      w.style.transitionTimingFunction = isBlack ? "cubic-bezier(0.22, 1, 0.36, 1)" : "cubic-bezier(0.76, 0, 0.24, 1)";
    });
    requestAnimationFrame(() => requestAnimationFrame(() => waves.forEach((w) => w.classList.add("out"))));
    const t = window.setTimeout(() => stack.remove(), 1900);
    return () => { window.clearTimeout(t); window.clearTimeout(clear); stack.remove(); delete document.body.dataset.enter; };
  }, []);
  return null;
}
