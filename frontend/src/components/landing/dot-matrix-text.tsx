"use client";
import { useEffect, useRef } from "react";

/**
 * Dot-matrix headline on canvas. Always alive: a soft wave of size/brightness travels through the glyphs;
 * the cursor gently pushes nearby dots. Inherits font-size/family/weight from the parent heading.
 */
export function DotMatrixText({ text, className = "" }: { text: string; className?: string }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const wrapRef = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const wrap = wrapRef.current;
    if (!canvas || !wrap) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    let dots: Array<{ x: number; y: number; p: number }> = [];
    let W = 1, H = 1, dpr = 1, base = 2, lastKey = "";
    const mouse = { x: -1e4, y: -1e4, tx: -1e4, ty: -1e4 };
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    let raf = 0;
    const t0 = performance.now();

    const build = () => {
      const cs = getComputedStyle(wrap);
      const fontPx = parseFloat(cs.fontSize);
      const key = `${fontPx}|${cs.fontFamily}|${cs.fontWeight}`;
      if (key === lastKey || !fontPx) return;
      lastKey = key;
      dpr = Math.min(2, window.devicePixelRatio || 1);
      const off = document.createElement("canvas");
      const octx = off.getContext("2d");
      if (!octx) return;
      const font = `${cs.fontWeight} ${fontPx}px ${cs.fontFamily}`;
      octx.font = font;
      const m = octx.measureText(text);
      W = Math.ceil(m.width + fontPx * 0.08);
      H = Math.ceil(fontPx * 1.18);
      off.width = W; off.height = H;
      octx.font = font;
      octx.fillStyle = "#000";
      octx.textBaseline = "alphabetic";
      octx.fillText(text, fontPx * 0.04, fontPx * 0.86);
      const img = octx.getImageData(0, 0, W, H).data;
      const step = Math.max(4, Math.round(fontPx / 12.5));
      base = step * 0.34;
      dots = [];
      for (let y = step / 2; y < H; y += step) {
        for (let x = step / 2; x < W; x += step) {
          const i = ((y | 0) * W + (x | 0)) * 4 + 3;
          if (img[i] > 110) dots.push({ x, y, p: Math.random() * Math.PI * 2 });
        }
      }
      canvas.width = Math.round(W * dpr);
      canvas.height = Math.round(H * dpr);
      canvas.style.width = `${W}px`;
      canvas.style.height = `${H}px`;
    };

    const draw = (now: number) => {
      const t = (now - t0) / 1000;
      // smooth cursor
      mouse.x += (mouse.tx - mouse.x) * 0.18;
      mouse.y += (mouse.ty - mouse.y) * 0.18;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, W, H);
      for (const d of dots) {
        const wave = 0.5 + 0.5 * Math.sin(t * 1.4 - d.x * 0.011 + d.y * 0.018 + d.p * 0.15);
        const dx = d.x - mouse.x, dy = d.y - mouse.y;
        const dist = Math.hypot(dx, dy);
        const inf = Math.max(0, 1 - dist / 150);
        const push = inf * inf * 9;
        const r = base * (0.55 + 0.55 * wave) * (1 + 0.5 * inf);
        ctx.globalAlpha = 0.5 + 0.5 * wave;
        ctx.fillStyle = inf > 0.05 ? "#2a5ebb" : "#2f6ad1";
        ctx.beginPath();
        ctx.arc(d.x + (dist ? (dx / dist) * push : 0), d.y + (dist ? (dy / dist) * push : 0), r, 0, Math.PI * 2);
        ctx.fill();
      }
      ctx.globalAlpha = 1;
      if (!reduced) raf = requestAnimationFrame(draw);
    };

    build();
    const fontsReady = (document as Document & { fonts?: { ready: Promise<unknown> } }).fonts?.ready;
    fontsReady?.then(() => { lastKey = ""; build(); if (reduced) draw(performance.now()); });
    if (reduced) draw(performance.now()); else raf = requestAnimationFrame(draw);

    const ro = new ResizeObserver(() => build());
    ro.observe(document.documentElement);
    const onMove = (e: MouseEvent) => { const r = canvas.getBoundingClientRect(); mouse.tx = e.clientX - r.left; mouse.ty = e.clientY - r.top; };
    const onLeave = () => { mouse.tx = -1e4; mouse.ty = -1e4; };
    window.addEventListener("mousemove", onMove, { passive: true });
    document.addEventListener("mouseleave", onLeave);
    return () => { cancelAnimationFrame(raf); ro.disconnect(); window.removeEventListener("mousemove", onMove); document.removeEventListener("mouseleave", onLeave); };
  }, [text]);

  return (
    <span ref={wrapRef} className={"inline-block align-top leading-none " + className}>
      <canvas ref={canvasRef} role="img" aria-label={text} className="block" />
    </span>
  );
}
