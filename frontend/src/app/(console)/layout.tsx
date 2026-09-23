import { ConsoleShell } from "@/components/app/console-shell";

/** Platform routes (/call, /admin) — Speko console, dark-first. */
export default function ConsoleLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="dark flex min-h-screen flex-1 flex-col bg-background text-foreground">
      <ConsoleShell>{children}</ConsoleShell>
    </div>
  );
}
