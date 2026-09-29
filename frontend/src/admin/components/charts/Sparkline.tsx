import { useId, useState } from "react";
import { cn } from "@/lib/utils";

export type SparkPoint = { x: string; y: number | null };

type SparklineProps = {
  points: SparkPoint[];
  /** Accessible summary, e.g. "Latency, 24h: 80–140 ms". */
  label: string;
  formatValue: (y: number) => string;
  formatX: (x: string) => string;
  /** Shared y-domain so small multiples compare; defaults to the series max. */
  max?: number;
  className?: string;
};

const W = 240;
const H = 40;
const PAD = 4;

/**
 * Single-series sparkline: 2px line in the accent hue, >=8px end marker with a
 * surface ring, crosshair + tooltip on hover, nulls break the line. One series
 * per panel means no legend and no categorical palette is needed.
 */
export function Sparkline({ points, label, formatValue, formatX, max, className }: SparklineProps) {
  const id = useId();
  const [active, setActive] = useState<number | null>(null);
  const values = points.map((p) => p.y).filter((y): y is number => y !== null);
  if (values.length === 0) {
    return <span className="text-xs text-muted-foreground">No data</span>;
  }
  const yMax = Math.max(max ?? 0, ...values, 1);
  const n = points.length;
  const sx = (i: number) => PAD + (i * (W - 2 * PAD)) / Math.max(1, n - 1);
  const sy = (y: number) => H - PAD - (y / yMax) * (H - 2 * PAD);

  const segments: string[] = [];
  let seg: string[] = [];
  points.forEach((p, i) => {
    if (p.y === null) {
      if (seg.length) segments.push(seg.join(" "));
      seg = [];
      return;
    }
    seg.push(`${seg.length === 0 ? "M" : "L"}${sx(i).toFixed(1)},${sy(p.y).toFixed(1)}`);
  });
  if (seg.length) segments.push(seg.join(" "));

  let lastIdx = -1;
  for (let i = n - 1; i >= 0; i--) {
    if (points[i].y !== null) {
      lastIdx = i;
      break;
    }
  }
  const hover = active !== null && points[active]?.y !== null ? active : null;

  return (
    <div className={cn("relative", className)}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        preserveAspectRatio="none"
        role="img"
        aria-label={label}
        className="h-10 w-full overflow-visible"
        onPointerMove={(e) => {
          const rect = e.currentTarget.getBoundingClientRect();
          const rel = (e.clientX - rect.left) / rect.width;
          setActive(Math.max(0, Math.min(n - 1, Math.round(rel * (n - 1)))));
        }}
        onPointerLeave={() => setActive(null)}
      >
        <title>{label}</title>
        {segments.map((d, i) => (
          <path
            key={`${id}-${i}`}
            d={d}
            fill="none"
            stroke="var(--chart-1)"
            strokeWidth={2}
            strokeLinejoin="round"
            strokeLinecap="round"
            vectorEffect="non-scaling-stroke"
          />
        ))}
        {lastIdx >= 0 && points[lastIdx].y !== null ? (
          <circle
            cx={sx(lastIdx)}
            cy={sy(points[lastIdx].y as number)}
            r={4}
            fill="var(--chart-1)"
            stroke="var(--card)"
            strokeWidth={2}
            vectorEffect="non-scaling-stroke"
          />
        ) : null}
        {hover !== null ? (
          <>
            <line
              x1={sx(hover)}
              x2={sx(hover)}
              y1={0}
              y2={H}
              stroke="var(--border-strong)"
              strokeWidth={1}
              vectorEffect="non-scaling-stroke"
            />
            <circle
              cx={sx(hover)}
              cy={sy(points[hover].y as number)}
              r={4}
              fill="var(--chart-1)"
              stroke="var(--card)"
              strokeWidth={2}
              vectorEffect="non-scaling-stroke"
            />
          </>
        ) : null}
      </svg>
      {hover !== null ? (
        <div
          role="status"
          className="pointer-events-none absolute top-0 z-10 rounded-md border border-border bg-popover px-2 py-1 text-xs whitespace-nowrap text-popover-foreground shadow-md"
          style={{
            left: `${(hover / Math.max(1, n - 1)) * 100}%`,
            transform: "translate(-50%, -110%)",
          }}
        >
          <span className="font-semibold">{formatValue(points[hover].y as number)}</span>{" "}
          <span className="text-muted-foreground">{formatX(points[hover].x)}</span>
        </div>
      ) : null}
    </div>
  );
}
