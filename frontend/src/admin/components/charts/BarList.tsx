import { Link } from "react-router";
import { cn } from "@/lib/utils";

export type BarItem = { key: string; label: string; value: number; to?: string; note?: string };

type BarListProps = {
  items: BarItem[];
  /** Accessible summary for the whole list. */
  label: string;
  formatValue?: (v: number) => string;
  /** Upper bound of the scale; defaults to the largest value. */
  max?: number;
  className?: string;
};

/**
 * Horizontal bar list for magnitude across categories: one hue, thin bars,
 * value as a direct label, category as text. Sorted by the caller.
 */
export function BarList({ items, label, formatValue = String, max, className }: BarListProps) {
  const top = Math.max(max ?? 0, ...items.map((i) => i.value), 1);
  return (
    <ul aria-label={label} className={cn("flex flex-col gap-2", className)}>
      {items.map((it) => (
        <li
          key={it.key}
          className="grid grid-cols-[minmax(0,10rem)_1fr_auto] items-center gap-3 text-sm"
        >
          {it.to ? (
            <Link to={it.to} className="truncate text-foreground hover:underline">
              {it.label}
            </Link>
          ) : (
            <span className="truncate text-foreground">{it.label}</span>
          )}
          <div
            className="h-2 w-full rounded-sm bg-muted"
            role="img"
            aria-label={`${it.label}: ${formatValue(it.value)}`}
            title={it.note ? `${formatValue(it.value)} · ${it.note}` : formatValue(it.value)}
          >
            <div
              className="h-full rounded-sm bg-(--chart-1)"
              style={{ width: `${Math.max(1, (it.value / top) * 100)}%` }}
            />
          </div>
          <span className="text-muted-foreground tabular-nums">{formatValue(it.value)}</span>
        </li>
      ))}
    </ul>
  );
}
