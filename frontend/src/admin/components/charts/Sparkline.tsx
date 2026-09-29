import { Line, LineChart, Tooltip, YAxis } from "recharts";
import { type ChartConfig, ChartContainer } from "@/components/ui/chart";
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

const config = {
  y: { label: "Latency", color: "var(--chart-1)" },
} satisfies ChartConfig;

/**
 * Single-series sparkline on Recharts: 2px line in the accent hue, a marker
 * on hover, nulls break the line. One series per panel means no legend and
 * no categorical palette is needed.
 */
export function Sparkline({ points, label, formatValue, formatX, max, className }: SparklineProps) {
  const values = points.map((p) => p.y).filter((y): y is number => y !== null);
  if (values.length === 0) {
    return <span className="text-xs text-muted-foreground">No data</span>;
  }
  const yMax = Math.max(max ?? 0, ...values, 1);
  return (
    <div role="img" aria-label={label} className={cn("w-full", className)}>
      <ChartContainer config={config} className="aspect-auto h-10 w-full">
        <LineChart data={points} margin={{ top: 4, right: 6, bottom: 4, left: 6 }}>
          <YAxis domain={[0, yMax]} hide />
          <Tooltip
            cursor={{ stroke: "var(--border-strong)", strokeWidth: 1 }}
            content={({ active, payload }) => {
              const p = payload?.[0]?.payload as SparkPoint | undefined;
              if (!active || !p || p.y === null) return null;
              return (
                <div className="rounded-md border border-border bg-popover px-2 py-1 text-xs text-popover-foreground shadow-md">
                  <span className="font-semibold">{formatValue(p.y)}</span>{" "}
                  <span className="text-muted-foreground">{formatX(p.x)}</span>
                </div>
              );
            }}
          />
          <Line
            type="monotone"
            dataKey="y"
            stroke="var(--color-y)"
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4, stroke: "var(--card)", strokeWidth: 2 }}
            connectNulls={false}
            isAnimationActive={false}
          />
        </LineChart>
      </ChartContainer>
    </div>
  );
}
