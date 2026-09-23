import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export function PageHeader({ title, actions, meta, className }: { title: ReactNode; actions?: ReactNode; meta?: ReactNode; className?: string }) {
  return (
    <div className={cn("flex min-h-[40px] flex-wrap items-center justify-between gap-3", className)}>
      <div className="flex items-center gap-3">
        <h1 className="text-[22px] font-[450] leading-none tracking-[-0.035em]">{title}</h1>
        {meta}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}

export function PageBody({ children, className, wide }: { children: ReactNode; className?: string; wide?: boolean }) {
  return <div className={cn("mx-auto w-full px-4 py-5 sm:px-8", wide ? "max-w-[1440px]" : "max-w-[1280px]", className)}>{children}</div>;
}
