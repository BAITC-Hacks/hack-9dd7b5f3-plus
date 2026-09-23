import type { Metadata } from "next";
import { Geist, Geist_Mono, Unbounded } from "next/font/google";
import "./globals.css";
import { cn } from "@/lib/utils";

// Geist Regular everywhere; Geist Mono only for numbers and ids.
const geist = Geist({ subsets: ["latin", "cyrillic"], variable: "--font-geist", weight: ["400", "500", "600"], display: "swap" });
const geistMono = Geist_Mono({ subsets: ["latin", "cyrillic"], variable: "--font-geist-mono", display: "swap" });
const unbounded = Unbounded({ subsets: ["latin", "cyrillic"], weight: ["700"], variable: "--font-unbounded", display: "swap" });

export const metadata: Metadata = {
  title: "Bagyt — Voice Router",
  description: "Голосовой робот контакт-центра с LLM-слоем выбора сценария. Русский и казахский, трассировка для супервизора.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="ru" className={cn("h-full antialiased font-sans", geist.variable, geistMono.variable, unbounded.variable)}>
      <body className="min-h-full flex flex-col">{children}</body>
    </html>
  );
}
