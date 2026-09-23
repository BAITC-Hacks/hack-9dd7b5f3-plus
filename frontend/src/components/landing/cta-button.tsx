"use client";
/** Brand-coloured ObsidianUI arrow-fill button. Clicking grows a circle from the button and hands over to the next page. */
import { useRouter } from "next/navigation";
import { ArrowFillButton } from "@/components/block/arrow-fill-button";
import { buildWaveStack } from "@/components/app/page-waves";

type Tone = "brand" | "ink" | "paper";
const TONES: Record<Tone, Record<string, string>> = {
  brand: { bgColor: "#2f6ad1", textColor: "#ffffff", fillBgColor: "#ffffff", fillTextColor: "#2f6ad1", hoverFillBgColor: "#0e1512", hoverFillTextColor: "#ffffff" },
  ink: { bgColor: "#0e1512", textColor: "#ffffff", fillBgColor: "#ffffff", fillTextColor: "#0e1512", hoverFillBgColor: "#2f6ad1", hoverFillTextColor: "#ffffff" },
  paper: { bgColor: "#ffffff", textColor: "#0e1512", fillBgColor: "#0e1512", fillTextColor: "#ffffff", hoverFillBgColor: "#2f6ad1", hoverFillTextColor: "#ffffff" },
};

export function wavesTo(router: ReturnType<typeof useRouter>, href: string) {
  const stack = buildWaveStack("start");
  requestAnimationFrame(() => requestAnimationFrame(() => { stack.querySelectorAll<HTMLElement>(".wave").forEach((w) => w.classList.add("in")); }));
  try { sessionStorage.setItem("bagyt:enter", "1"); } catch { /* noop */ }
  window.setTimeout(() => router.push(href), 950);
  window.setTimeout(() => stack.remove(), 9000); // safety: the next page removes it on mount
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
        wavesTo(router, href);
      }}
      {...TONES[tone]}
    >
      {children}
    </ArrowFillButton>
  );
}
