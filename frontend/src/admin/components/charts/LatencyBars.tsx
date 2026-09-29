import { Bar, BarChart, LabelList, XAxis, YAxis } from "recharts";
import { type ChartConfig, ChartContainer } from "@/components/ui/chart";

export type LatencyBarItem = { name: string; value: number };

const config = {
  value: { label: "p95 latency", color: "var(--chart-1)" },
} satisfies ChartConfig;

/**
 * Horizontal bars of one measure across services: one hue, thin bars, the
 * value as a direct label, services as text. Sorted by the caller.
 */
export function LatencyBars({ items, label }: { items: LatencyBarItem[]; label: string }) {
  const height = Math.max(96, items.length * 28 + 16);
  return (
    <div role="img" aria-label={label}>
      <ChartContainer config={config} className="aspect-auto w-full" style={{ height }}>
        <BarChart data={items} layout="vertical" margin={{ top: 0, right: 48, bottom: 0, left: 0 }}>
          <XAxis type="number" hide domain={[0, "dataMax"]} />
          <YAxis
            type="category"
            dataKey="name"
            width={140}
            tickLine={false}
            axisLine={false}
            tick={{ fill: "var(--foreground)", fontSize: 12 }}
          />
          <Bar
            dataKey="value"
            fill="var(--color-value)"
            radius={[0, 4, 4, 0]}
            barSize={10}
            isAnimationActive={false}
          >
            <LabelList
              dataKey="value"
              position="right"
              formatter={(v) => `${String(v)} ms`}
              className="fill-muted-foreground"
              fontSize={12}
            />
          </Bar>
        </BarChart>
      </ChartContainer>
    </div>
  );
}
