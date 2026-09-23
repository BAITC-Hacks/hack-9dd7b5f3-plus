import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export interface SegmentOption<T extends string> {
  value: T;
  label: ReactNode;
  disabled?: boolean;
  title?: string;
}

export function Segmented<T extends string>({
  value,
  onChange,
  options,
  className,
  size = "md",
  ariaLabel,
}: {
  value: T;
  onChange: (v: T) => void;
  options: SegmentOption<T>[];
  className?: string;
  size?: "sm" | "md";
  ariaLabel?: string;
}) {
  return (
    <div
      role="radiogroup"
      aria-label={ariaLabel}
      className={cn("inline-flex items-center rounded-[10px] border border-border bg-muted p-0.5", size === "sm" ? "text-[11.5px]" : "text-[12px]", className)}
    >
      {options.map((o) => {
        const active = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={active}
            disabled={o.disabled}
            title={o.title}
            onClick={() => onChange(o.value)}
            className={cn(
              "whitespace-nowrap rounded-[8px] font-medium transition-[background-color,color,transform] duration-150 active:scale-[0.97] disabled:cursor-not-allowed disabled:opacity-40",
              size === "sm" ? "h-6 px-2" : "h-7 px-2.5",
              active ? "bg-accent text-foreground" : "text-muted-foreground hover:text-foreground",
            )}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
