"use client";
/** Brand-coloured ObsidianUI arrow-fill button, used for every landing CTA. */
import Link from "next/link";
import { ArrowFillButton } from "@/components/block/arrow-fill-button";

type Tone = "brand" | "ink" | "paper";
const TONES: Record<Tone, Record<string, string>> = {
  brand: { bgColor: "#2f6ad1", textColor: "#ffffff", fillBgColor: "#ffffff", fillTextColor: "#2f6ad1", hoverFillBgColor: "#0e1512", hoverFillTextColor: "#ffffff" },
  ink: { bgColor: "#0e1512", textColor: "#ffffff", fillBgColor: "#ffffff", fillTextColor: "#0e1512", hoverFillBgColor: "#2f6ad1", hoverFillTextColor: "#ffffff" },
  paper: { bgColor: "#ffffff", textColor: "#0e1512", fillBgColor: "#0e1512", fillTextColor: "#ffffff", hoverFillBgColor: "#2f6ad1", hoverFillTextColor: "#ffffff" },
};

export function CtaButton({ href, children, tone = "brand", className }: { href: "/call" | "/admin" | "/"; children: React.ReactNode; tone?: Tone; className?: string }) {
  return (
    <ArrowFillButton as={Link} href={href} className={className} {...TONES[tone]}>
      {children}
    </ArrowFillButton>
  );
}
