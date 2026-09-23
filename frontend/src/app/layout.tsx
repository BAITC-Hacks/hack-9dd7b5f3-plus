import type { Metadata } from "next";
import "./globals.css";
export const metadata: Metadata = { title: "Plus / Voice Router", description: "Голосовой AI-маршрутизатор с наблюдаемыми решениями. HackAlem AI · Halyk Bank." };
export default function RootLayout({ children }: { children: React.ReactNode }) { return <html lang="ru"><body>{children}</body></html>; }
