"use client";

import { useEffect, type ReactNode } from "react";
import { X } from "lucide-react";
import { cn } from "@/lib/cn";
import { Button } from "./button";

export function Modal({
  open,
  onClose,
  title,
  children,
  wide,
  actions,
}: {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  children: ReactNode;
  wide?: boolean;
  actions?: ReactNode;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/70 p-4 backdrop-blur-[2px]" onClick={onClose}>
      <div
        role="dialog"
        aria-modal="true"
        onClick={(e) => e.stopPropagation()}
        className={cn(
          "rise flex max-h-[85vh] w-full flex-col overflow-hidden rounded-[15px] border border-border bg-popover",
          wide ? "max-w-4xl" : "max-w-xl",
        )}
      >
        <header className="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
          <h2 className="text-[15px] font-[450] tracking-[-0.03em]">{title}</h2>
          <div className="flex items-center gap-2">
            {actions}
            <Button variant="ghost" size="icon-sm" onClick={onClose} aria-label="Close">
              <X className="size-4" />
            </Button>
          </div>
        </header>
        <div className="min-h-0 flex-1 overflow-auto p-4">{children}</div>
      </div>
    </div>
  );
}
