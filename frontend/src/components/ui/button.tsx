import type { ButtonHTMLAttributes } from "react";
import { cn } from "@/lib/cn";

export type ButtonVariant = "primary" | "outline" | "ghost" | "danger" | "subtle";
export type ButtonSize = "sm" | "md" | "lg" | "icon" | "icon-sm";

const base =
  "inline-flex select-none items-center justify-center gap-1.5 whitespace-nowrap rounded-[10px] text-[12.5px] font-medium tracking-[-0.01em] transition-[background-color,border-color,color,transform,opacity] duration-150 active:scale-[0.97] disabled:pointer-events-none disabled:opacity-40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50";

const variants: Record<ButtonVariant, string> = {
  primary: "bg-primary text-primary-foreground hover:bg-primary/90",
  outline: "border border-border bg-transparent text-foreground/80 hover:bg-foreground/[0.05] hover:text-foreground",
  ghost: "text-muted-foreground hover:bg-foreground/[0.05] hover:text-foreground",
  danger: "border border-destructive/30 bg-destructive/10 text-destructive hover:bg-destructive/15",
  subtle: "border border-primary/25 bg-primary/10 text-primary hover:bg-primary/15",
};

const sizes: Record<ButtonSize, string> = {
  sm: "h-7 px-2.5 text-[12px]",
  md: "h-8 px-3",
  lg: "h-9 px-4 text-[13px]",
  icon: "size-8 p-0",
  "icon-sm": "size-7 p-0",
};

export function Button({
  variant = "outline",
  size = "md",
  className,
  type = "button",
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant; size?: ButtonSize }) {
  return <button type={type} className={cn(base, variants[variant], sizes[size], className)} {...rest} />;
}
