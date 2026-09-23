import { cn } from "@/lib/utils";

/** Logo: a blue tile with a dot-matrix plus (the dither motif) + wordmark in Unbounded. */
export function LogoMark({ className = "size-7", dark = false }: { className?: string; dark?: boolean }) {
  const dots: Array<[number, number]> = [[10, 4], [4, 10], [10, 10], [16, 10], [10, 16]];
  return (
    <span aria-hidden className={cn("grid shrink-0 place-items-center rounded-[7px]", dark ? "bg-[#f5f5f5]" : "bg-brand", className)}>
      <svg viewBox="0 0 20 20" className="size-[70%]" fill={dark ? "#0d0d0d" : "#ffffff"}>
        {dots.map(([x, y]) => <circle key={`${x}-${y}`} cx={x} cy={y} r="2.1" />)}
      </svg>
    </span>
  );
}

export function Wordmark({ className = "text-[17px]" }: { className?: string }) {
  return <span className={cn("font-logo font-bold tracking-[-0.04em]", className)}>Bagyt</span>;
}
