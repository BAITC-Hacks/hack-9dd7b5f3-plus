import { cn } from "@/lib/cn";

export function JsonView({ value, className, maxHeight = "max-h-72" }: { value: unknown; className?: string; maxHeight?: string }) {
  let text: string;
  try {
    text = typeof value === "string" ? value : JSON.stringify(value, null, 2);
  } catch {
    text = String(value);
  }
  return (
    <pre
      className={cn(
        "overflow-auto rounded-[10px] border border-border bg-background/60 p-3 font-mono text-[11px] leading-[1.55] tracking-normal whitespace-pre-wrap break-words text-foreground/80",
        maxHeight,
        className,
      )}
    >
      {text}
    </pre>
  );
}
