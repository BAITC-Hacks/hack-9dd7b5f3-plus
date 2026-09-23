import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export function Panel({
  title,
  note,
  actions,
  children,
  className,
  bodyClassName,
  id,
}: {
  title?: ReactNode;
  note?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  bodyClassName?: string;
  id?: string;
}) {
  return (
    <section id={id} className={cn("relative min-w-0 rounded-[15px] border border-border bg-card", className)}>
      {(title || actions || note) && (
        <header className="flex min-h-[42px] items-center justify-between gap-3 border-b border-border px-4 py-2">
          <h2 className="truncate text-[13.5px] font-[450] tracking-[-0.02em]">{title}</h2>
          <div className="flex shrink-0 items-center gap-2">
            {note && <span className="text-[11px] tabular-nums text-muted-foreground/60">{note}</span>}
            {actions}
          </div>
        </header>
      )}
      <div className={cn("p-4", bodyClassName)}>{children}</div>
    </section>
  );
}

export function Label({ children, className }: { children: ReactNode; className?: string }) {
  return <p className={cn("text-[10.5px] text-muted-foreground/55", className)}>{children}</p>;
}

export function Section({ label, children, className, right }: { label: ReactNode; children: ReactNode; className?: string; right?: ReactNode }) {
  return (
    <div className={cn("space-y-1.5", className)}>
      <div className="flex items-center justify-between gap-2">
        <Label>{label}</Label>
        {right}
      </div>
      {children}
    </div>
  );
}
