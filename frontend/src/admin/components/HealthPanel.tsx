import { Link } from "react-router";
import { StatusBadge } from "@/components/StatusBadge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import type { AdminHealthSeriesResponse } from "../api/types";
import { Sparkline } from "./charts/Sparkline";
import { EmptyState } from "./EmptyState";
import { formatUptime, HealthBucketStrip } from "./HealthStrip";

const TIME = new Intl.DateTimeFormat(undefined, { hour: "numeric", minute: "2-digit" });

type HealthPanelProps = {
  data?: AdminHealthSeriesResponse;
  loading: boolean;
  error?: Error | null;
  hours?: number;
};

/**
 * Small multiples: one row per probed service with its uptime strip and a
 * latency sparkline on a shared 24h window and a shared latency scale, so the
 * rows compare at a glance. One series per panel, no legend needed.
 */
export function HealthPanel({ data, loading, error, hours = 24 }: HealthPanelProps) {
  const services = data?.services ?? [];
  const latencyMax = Math.max(
    1,
    ...services.flatMap((s) => s.buckets.map((b) => b.p50LatencyMs ?? 0)),
  );
  const window = data
    ? `${TIME.format(new Date(data.from))} – ${TIME.format(new Date(data.to))}`
    : "";

  return (
    <Card>
      <CardHeader>
        <CardTitle>Service health and response time</CardTitle>
        <CardDescription>
          Last {hours === 1 ? "hour" : `${hours} hours`} in {data?.buckets ?? 48} periods
          {window ? ` (${window})` : ""}. Uptime strip on top, median probe latency below, on a
          shared scale of 0–{latencyMax} ms.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="flex flex-col gap-3">
            <Skeleton className="h-16" shape="block" />
            <Skeleton className="h-16" shape="block" />
          </div>
        ) : error ? (
          <EmptyState
            variant="error"
            title="Couldn't load health history"
            description={error.message}
          />
        ) : services.length === 0 ? (
          <EmptyState
            title="No probed services"
            description="Enable landingPage.healthCheck on a NebariApp to see it here."
          />
        ) : (
          <ul className="grid gap-x-8 gap-y-5 lg:grid-cols-2">
            {services.map((s) => (
              <li key={s.id} className="flex flex-col gap-1.5">
                <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
                  <span className="flex items-center gap-2">
                    <Link
                      to={`/admin/services/${encodeURIComponent(s.id)}`}
                      className="font-medium hover:underline"
                    >
                      {s.displayName}
                    </Link>
                    <StatusBadge status={s.status} />
                  </span>
                  <span className="text-xs text-muted-foreground tabular-nums">
                    {formatUptime(s.uptimePercent)} uptime
                    {s.latencyMs !== null ? ` · ${s.latencyMs} ms now` : ""}
                  </span>
                </div>
                <HealthBucketStrip buckets={s.buckets} />
                <Sparkline
                  points={s.buckets.map((b) => ({ x: b.start, y: b.p50LatencyMs }))}
                  max={latencyMax}
                  label={`${s.displayName} median latency over the last ${hours === 1 ? "hour" : `${hours} hours`}`}
                  formatValue={(y) => `${y} ms`}
                  formatX={(x) => TIME.format(new Date(x))}
                />
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
