"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { AudioLines, BookOpen, FlaskConical, LayoutDashboard, Terminal, type LucideIcon } from "lucide-react";
import { cn } from "@/lib/cn";
import { BackendStatus } from "./backend-status";

const NAV: Array<{ href: string; label: string; icon: LucideIcon }> = [
  { href: "/", label: "Call", icon: AudioLines },
  { href: "/supervisor", label: "Supervisor", icon: LayoutDashboard },
  { href: "/eval", label: "Eval", icon: FlaskConical },
  { href: "/catalog", label: "Catalog", icon: BookOpen },
  { href: "/debug", label: "Debug", icon: Terminal },
];

function isActive(pathname: string, href: string): boolean {
  if (href === "/") return pathname === "/";
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function Sidebar() {
  const pathname = usePathname() ?? "/";
  return (
    <aside className="hidden w-[224px] shrink-0 flex-col border-r border-sidebar-border bg-sidebar md:flex">
      <div className="flex h-14 items-center gap-2.5 px-4">
        <span className="grid size-7 place-items-center rounded-[9px] bg-primary/[0.14] text-primary">
          <AudioLines className="size-4" strokeWidth={2} />
        </span>
        <div className="min-w-0 leading-tight">
          <p className="truncate text-[13.5px] font-[450] tracking-[-0.02em]">Voice Router</p>
          <p className="truncate text-[10.5px] text-muted-foreground/55">Saqta Insurance · contact center</p>
        </div>
      </div>
      <nav className="flex min-h-0 flex-1 flex-col px-3 pb-2">
        <p className="mb-1 mt-2 px-2.5 text-[11px] text-muted-foreground/45">Console</p>
        <div className="flex flex-col gap-0.5">
          {NAV.map((item) => {
            const active = isActive(pathname, item.href);
            const Icon = item.icon;
            return (
              <Link
                key={item.href}
                href={item.href}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "group relative flex h-9 select-none items-center gap-2.5 rounded-[10px] px-2.5 transition-[background-color,color,transform] duration-150 active:scale-[0.98]",
                  active ? "bg-primary/[0.1]" : "hover:bg-foreground/[0.045]",
                )}
              >
                {active && <span className="absolute -left-3 top-1/2 h-4 w-[3px] -translate-y-1/2 rounded-r bg-primary" />}
                <Icon className={cn("size-[18px]", active ? "text-primary" : "text-muted-foreground/70 group-hover:text-foreground/80")} strokeWidth={1.75} />
                <span className={cn("flex-1 truncate text-[13.5px] tracking-[-0.02em]", active ? "font-medium text-foreground" : "text-muted-foreground")}>
                  {item.label}
                </span>
              </Link>
            );
          })}
        </div>
      </nav>
      <div className="border-t border-sidebar-border p-4">
        <BackendStatus />
      </div>
    </aside>
  );
}

export function MobileNav() {
  const pathname = usePathname() ?? "/";
  return (
    <nav className="fixed inset-x-0 bottom-0 z-40 flex items-center justify-around border-t border-border bg-background/90 py-1.5 backdrop-blur-md md:hidden">
      {NAV.map((item) => {
        const active = isActive(pathname, item.href);
        const Icon = item.icon;
        return (
          <Link
            key={item.href}
            href={item.href}
            aria-current={active ? "page" : undefined}
            className={cn("flex flex-col items-center gap-0.5 rounded-[9px] px-3 py-1 active:scale-[0.94]", active ? "text-primary" : "text-muted-foreground")}
          >
            <Icon className="size-5" strokeWidth={1.75} />
            <span className={cn("text-[9.5px]", active && "font-medium")}>{item.label}</span>
          </Link>
        );
      })}
    </nav>
  );
}

export function MobileHeader() {
  return (
    <header className="flex h-12 items-center justify-between border-b border-border px-4 md:hidden">
      <div className="flex items-center gap-2">
        <span className="grid size-6 place-items-center rounded-[7px] bg-primary/[0.14] text-primary">
          <AudioLines className="size-3.5" strokeWidth={2} />
        </span>
        <span className="text-[13px] font-[450] tracking-[-0.02em]">Voice Router</span>
      </div>
      <BackendStatus compact />
    </header>
  );
}
