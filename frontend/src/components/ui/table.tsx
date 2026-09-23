import type { ReactNode, TableHTMLAttributes, TdHTMLAttributes, ThHTMLAttributes } from "react";
import { cn } from "@/lib/cn";

export function Table({ className, children, ...rest }: TableHTMLAttributes<HTMLTableElement> & { children: ReactNode }) {
  return (
    <div className="w-full overflow-x-auto">
      <table className={cn("w-full border-collapse text-[12.5px]", className)} {...rest}>
        {children}
      </table>
    </div>
  );
}

export function Th({ className, children, ...rest }: ThHTMLAttributes<HTMLTableCellElement> & { children?: ReactNode }) {
  return (
    <th
      className={cn(
        "sticky top-0 z-[1] whitespace-nowrap border-b border-border bg-card px-3 py-2 text-left text-[10.5px] font-normal text-muted-foreground/55",
        className,
      )}
      {...rest}
    >
      {children}
    </th>
  );
}

export function Td({ className, children, ...rest }: TdHTMLAttributes<HTMLTableCellElement> & { children?: ReactNode }) {
  return (
    <td className={cn("border-b border-border/70 px-3 py-2 align-middle", className)} {...rest}>
      {children}
    </td>
  );
}
