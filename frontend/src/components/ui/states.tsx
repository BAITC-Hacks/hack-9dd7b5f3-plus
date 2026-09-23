import type { ReactNode } from "react";
import { AlertTriangle, Inbox, RefreshCw } from "lucide-react";
import { cn } from "@/lib/cn";
import { Button } from "./button";

export function EmptyState({
  icon,
  title,
  hint,
  action,
  className,
}: {
  icon?: ReactNode;
  title: ReactNode;
  hint?: ReactNode;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col items-center justify-center gap-2 px-4 py-10 text-center", className)}>
      <div className="grid size-10 place-items-center rounded-full bg-primary/10 text-primary ring-1 ring-primary/20">
        {icon ?? <Inbox className="size-4" />}
      </div>
      <p className="text-[13px] font-medium">{title}</p>
      {hint && <p className="max-w-sm text-[12px] leading-relaxed text-muted-foreground">{hint}</p>}
      {action && <div className="mt-1">{action}</div>}
    </div>
  );
}

export function LoadingState({ rows = 4, className }: { rows?: number; className?: string }) {
  return (
    <div className={cn("space-y-2.5 p-4", className)} aria-busy="true" aria-label="Loading">
      {Array.from({ length: rows }).map((_, i) => (
        <div
          key={i}
          className="h-3 animate-pulse rounded bg-foreground/[0.06]"
          style={{ width: `${[92, 70, 84, 58, 76, 64][i % 6]}%` }}
        />
      ))}
    </div>
  );
}

export function ErrorState({ message, onRetry, className }: { message: string; onRetry?: () => void; className?: string }) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-2 rounded-[15px] border border-destructive/25 bg-destructive/[0.04] px-4 py-8 text-center",
        className,
      )}
      role="alert"
    >
      <AlertTriangle className="size-4 text-destructive" />
      <p className="text-[13px] font-medium text-destructive">Something went wrong</p>
      <p className="max-w-md text-[12px] leading-relaxed text-muted-foreground">{message}</p>
      {onRetry && (
        <Button size="sm" onClick={onRetry} className="mt-1">
          <RefreshCw className="size-3.5" /> Retry
        </Button>
      )}
    </div>
  );
}
