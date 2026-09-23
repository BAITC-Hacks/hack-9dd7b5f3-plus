"use client";
import { useEffect } from "react";
import { buildWaveStack } from "./page-waves";

/** When the page was opened through a CTA wave, the sheets are already covering: send them upwards to reveal. */
export function EnterReveal() {
  useEffect(() => {
    let flagged = false;
    try { flagged = sessionStorage.getItem("bagyt:enter") === "1"; if (flagged) sessionStorage.removeItem("bagyt:enter"); } catch { /* noop */ }
    document.querySelectorAll(".wave-stack").forEach((el) => el.remove());
    if (!flagged) return;
    const stack = buildWaveStack("covering");
    const waves = [...stack.querySelectorAll<HTMLElement>(".wave")].reverse(); // blue leaves first, black last
    waves.forEach((w, i) => { w.style.transitionDelay = `${120 + i * 110}ms`; });
    requestAnimationFrame(() => requestAnimationFrame(() => waves.forEach((w) => w.classList.add("out"))));
    const t = window.setTimeout(() => stack.remove(), 1500);
    return () => { window.clearTimeout(t); stack.remove(); };
  }, []);
  return null;
}
