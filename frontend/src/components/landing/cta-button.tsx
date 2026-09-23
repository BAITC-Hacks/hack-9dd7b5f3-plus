"use client";
/** Brand-coloured ObsidianUI arrow-fill button. Clicking grows a circle from the button and hands over to the next page. */
import { useRouter } from "next/navigation";
import { ArrowFillButton } from "@/components/block/arrow-fill-button";

type Tone = "brand" | "ink" | "paper";
const TONES: Record<Tone, Record<string, string>> = {
  brand: { bgColor: "#2f6ad1", textColor: "#ffffff", fillBgColor: "#ffffff", fillTextColor: "#2f6ad1", hoverFillBgColor: "#0e1512", hoverFillTextColor: "#ffffff" },
  ink: { bgColor: "#0e1512", textColor: "#ffffff", fillBgColor: "#ffffff", fillTextColor: "#0e1512", hoverFillBgColor: "#2f6ad1", hoverFillTextColor: "#ffffff" },
  paper: { bgColor: "#ffffff", textColor: "#0e1512", fillBgColor: "#0e1512", fillTextColor: "#ffffff", hoverFillBgColor: "#2f6ad1", hoverFillTextColor: "#ffffff" },
};

export function wipeTo(router: ReturnType<typeof useRouter>, href: string, from: DOMRect) {
  const cx = from.left + from.width / 2;
  const cy = from.top + from.height / 2;
  const far = Math.max(Math.hypot(cx, cy), Math.hypot(window.innerWidth - cx, cy), Math.hypot(cx, window.innerHeight - cy), Math.hypot(window.innerWidth - cx, window.innerHeight - cy));
  const d = 40;
  const el = document.createElement("div");
  el.className = "page-wipe";
  el.style.left = `${cx}px`; el.style.top = `${cy}px`; el.style.width = `${d}px`; el.style.height = `${d}px`;
  el.style.setProperty("--k", String((far * 2) / d + 1));
  document.body.appendChild(el);
  requestAnimationFrame(() => { el.style.transform = `translate(-50%, -50%) scale(${(far * 2) / d + 1})`; });
  try { sessionStorage.setItem("bagyt:enter", "1"); } catch { /* noop */ }
  window.setTimeout(() => router.push(href), 520);
  window.setTimeout(() => el.remove(), 1600);
}

export function CtaButton({ href, children, tone = "brand", className }: { href: "/call" | "/admin" | "/"; children: React.ReactNode; tone?: Tone; className?: string }) {
  const router = useRouter();
  return (
    <ArrowFillButton
      as="a"
      href={href}
      className={className}
      onClick={(e: React.MouseEvent<HTMLElement>) => {
        if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
        e.preventDefault();
        wipeTo(router, href, (e.currentTarget as HTMLElement).getBoundingClientRect());
      }}
      {...TONES[tone]}
    >
      {children}
    </ArrowFillButton>
  );
}
