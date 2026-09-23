"use client";
import { useEffect, useState } from "react";

/** Plays the "reveal from blue" once when the page was opened through a CTA wipe. */
export function EnterReveal() {
  const [on, setOn] = useState(false);
  useEffect(() => {
    try {
      if (sessionStorage.getItem("bagyt:enter") === "1") {
        sessionStorage.removeItem("bagyt:enter");
        setOn(true);
        const t = window.setTimeout(() => setOn(false), 800);
        return () => window.clearTimeout(t);
      }
    } catch { /* noop */ }
  }, []);
  return on ? <div aria-hidden className="enter-reveal" /> : null;
}
