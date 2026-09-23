import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";
import { MobileHeader, MobileNav, Sidebar } from "@/components/shell/sidebar";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin", "cyrillic"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin", "cyrillic"],
});

export const metadata: Metadata = {
  title: { default: "Voice Router", template: "%s · Voice Router" },
  description: "Voice robot simulator for the Saqta Insurance contact center: LLM scenario routing with a supervisor trace panel.",
};

export const viewport: Viewport = {
  themeColor: "#0a0d15",
  colorScheme: "dark",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}>
      <body className="h-dvh overflow-hidden bg-background text-foreground">
        <div className="flex h-full">
          <Sidebar />
          <div className="flex min-w-0 flex-1 flex-col">
            <MobileHeader />
            <main className="min-h-0 flex-1 overflow-y-auto pb-16 md:pb-0">{children}</main>
          </div>
        </div>
        <MobileNav />
      </body>
    </html>
  );
}
