import type { Metadata } from "next";
import { Geist_Mono, Hanken_Grotesk, Inter } from "next/font/google";
import "./globals.css";
import { cn } from "@/lib/utils";

// Speko type system: Hanken Grotesk (display + body). It has no basic Cyrillic,
// so Inter (same grotesk family feel) is stacked behind it for Russian/Kazakh glyphs.
const hanken = Hanken_Grotesk({ subsets: ["latin", "latin-ext"], variable: "--font-hanken", weight: ["400", "500", "600", "700"], display: "swap" });
const inter = Inter({ subsets: ["latin", "cyrillic", "cyrillic-ext"], variable: "--font-inter", weight: ["400", "500", "600", "700"], display: "swap" });
const geistMono = Geist_Mono({ subsets: ["latin", "cyrillic"], variable: "--font-geist-mono", display: "swap" });

export const metadata: Metadata = {
  title: "Bagyt — Voice Router",
  description: "Голосовой робот контакт-центра с LLM-слоем выбора сценария. Русский и казахский, трассировка для супервизора.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="ru" className={cn("h-full antialiased font-sans", hanken.variable, inter.variable, geistMono.variable)}>
      <body className="min-h-full flex flex-col">{children}</body>
    </html>
  );
}
