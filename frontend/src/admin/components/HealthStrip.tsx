import { useMemo } from "react";
import { cn } from "@/lib/utils";
import type { HealthSample } from "../api/types";
import { formatDateTime } from "../lib/format";

type HealthStripProps = {
  samples: HealthSample[];
  /** Number of bars to render; samples are bucketed oldest→newest. */
  buckets?: number;
  className?: string;
};

type Bucket = { status: string; from: string; to: string; count: number };

/**
 * Uptime strip: one thin bar per time bucket, colored with the status tokens
 * the landing page already uses for its health badge. A bucket takes the
 * worst outcome it contains so a brief outage stays visible. Identity is not
 * color-alone: the strip carries an accessible summary and each bar a title.
 */
export function HealthStrip({ samples, buckets = 96, className }: HealthStripProps) {
  const bars = useMemo<Bucket[]>(() => {
    if (samples.length === 0) return [];
    const size = Math.max(1, Math.ceil(samples.length / buckets));
    const out: Bucket[] = [];
    for (let i = 0; i < samples.length; i += size) {
      const slice = samples.slice(i, i + size);
      const status = slice.some((s) => s.status === "unhealthy")
        ? "unhealthy"
        : slice.every((s) => s.status === "healthy")
          ? "healthy"
          : "unknown";
      out.push({ status, from: slice[0].at, to: slice[slice.length - 1].at, count: slice.length });
    }
    return out;
  }, [samples, buckets]);

  if (bars.length === 0) {
    return <p className="text-sm text-muted-foreground">No probes recorded yet.</p>;
  }

  const healthy = bars.filter((b) => b.status === "healthy").length;
  const unhealthy = bars.filter((b) => b.status === "unhealthy").length;

  return (
    <div
      role="img"
      aria-label={`Health over ${bars.length} periods from ${formatDateTime(bars[0].from)}: ${healthy} healthy, ${unhealthy} unhealthy, ${bars.length - healthy - unhealthy} unknown`}
      className={cn("flex h-8 w-full items-stretch gap-px", className)}
    >
      {bars.map((b) => (
        <span
          key={b.from}
          title={`${b.status} · ${formatDateTime(b.from)}${b.count > 1 ? ` – ${formatDateTime(b.to)}` : ""}`}
          className={cn(
            "min-w-px flex-1 rounded-sm",
            b.status === "healthy" && "bg-(--status-healthy-dot)",
            b.status === "unhealthy" && "bg-(--status-unhealthy-dot)",
            b.status === "unknown" && "bg-(--status-default-dot) opacity-50",
          )}
        />
      ))}
    </div>
  );
}

export function formatUptime(pct: number | null | undefined): string {
  if (pct === null || pct === undefined) return "—";
  return `${pct >= 99.95 ? "100" : pct.toFixed(pct >= 99 ? 2 : 1)}%`;
}
