import { useMemo, useState } from "react";
import { Link } from "react-router";
import { StatusBadge } from "@/components/StatusBadge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { DataTable, type DataTableColumnDef } from "@/components/ui/data-table";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsList, TabsTab } from "@/components/ui/tabs";
import type { HealthBucket, ServiceHealthSeries } from "../api/types";
import { LatencyBars } from "../components/charts/LatencyBars";
import { EmptyState } from "../components/EmptyState";
import { HealthPanel } from "../components/HealthPanel";
import { formatUptime, HealthBucketStrip } from "../components/HealthStrip";
import { PageHeader } from "../components/PageHeader";
import { StatTile } from "../components/StatTile";
import { useHealthSeries } from "../hooks/useAdminData";
import { formatDateTime, pluralize } from "../lib/format";

type Window = 1 | 6 | 24;
const BUCKETS: Record<Window, number> = { 1: 60, 6: 72, 24: 48 };

type Row = ServiceHealthSeries &
  Record<string, unknown> & { uptime: number; p50: number; p95: number; incidentCount: number };

function fmtDuration(fromIso: string, toIso: string | null): string {
  const ms = (toIso ? Date.parse(toIso) : Date.now()) - Date.parse(fromIso);
  const min = Math.max(1, Math.round(ms / 60000));
  if (min < 60) return `${min} min`;
  const h = Math.floor(min / 60);
  const m = min % 60;
  return m ? `${h} h ${m} min` : `${h} h`;
}

/**
 * Health section: a window switch, headline figures, the platform-wide uptime
 * strip (worst status across services per period), the per-service small
 * multiples, a scorecard table, and the incident log. All derived from the
 * webapi's retained probe history, so the window is capped at what it keeps.
 */
export function HealthPage() {
  const [hours, setHours] = useState<Window>(24);
  const health = useHealthSeries(hours, BUCKETS[hours]);
  const services = health.data?.services ?? [];

  const platform = useMemo<HealthBucket[]>(() => {
    if (services.length === 0) return [];
    const n = services[0].buckets.length;
    return Array.from({ length: n }, (_, i) => {
      const acc: HealthBucket = {
        start: services[0].buckets[i]?.start ?? "",
        total: 0,
        healthy: 0,
        unhealthy: 0,
        unknown: 0,
        p50LatencyMs: null,
      };
      for (const s of services) {
        const b = s.buckets[i];
        if (!b) continue;
        acc.total += b.total;
        acc.healthy += b.healthy;
        acc.unhealthy += b.unhealthy;
        acc.unknown += b.unknown;
      }
      return acc;
    });
  }, [services]);

  const totals = useMemo(() => {
    let probes = 0;
    let healthy = 0;
    for (const b of platform) {
      probes += b.total;
      healthy += b.healthy;
    }
    const incidents = services.flatMap((s) =>
      s.incidents.map((i) => ({ ...i, serviceId: s.id, displayName: s.displayName })),
    );
    incidents.sort((a, b) => b.from.localeCompare(a.from));
    const latencies = services.map((s) => s.p50LatencyMs).filter((v): v is number => v !== null);
    const medianNow = latencies.length
      ? [...latencies].sort((a, b) => a - b)[Math.floor(latencies.length / 2)]
      : null;
    return {
      uptime: probes ? (healthy * 100) / probes : null,
      healthyNow: services.filter((s) => s.status === "healthy").length,
      incidents,
      open: incidents.filter((i) => i.to === null).length,
      medianNow,
    };
  }, [platform, services]);

  const rows = useMemo<Row[]>(
    () =>
      services.map((s) => ({
        ...s,
        uptime: s.uptimePercent ?? -1,
        p50: s.p50LatencyMs ?? -1,
        p95: s.p95LatencyMs ?? -1,
        incidentCount: s.incidents.length,
      })),
    [services],
  );

  const columns = useMemo<DataTableColumnDef<Row>[]>(
    () => [
      {
        id: "displayName",
        accessorKey: "displayName",
        header: "Service",
        cell: ({ row }) => (
          <Link
            to={`/admin/services/${encodeURIComponent(row.original.id)}`}
            className="font-medium text-foreground hover:underline"
          >
            {row.original.displayName}
          </Link>
        ),
      },
      {
        id: "status",
        accessorKey: "status",
        header: "Now",
        cell: ({ row }) => <StatusBadge status={row.original.status} />,
      },
      {
        id: "uptime",
        accessorKey: "uptime",
        header: "Uptime",
        cell: ({ row }) => (
          <span className="tabular-nums">{formatUptime(row.original.uptimePercent)}</span>
        ),
      },
      {
        id: "p50",
        accessorKey: "p50",
        header: "p50 latency",
        cell: ({ row }) => (
          <span className="tabular-nums">
            {row.original.p50LatencyMs !== null ? `${row.original.p50LatencyMs} ms` : "—"}
          </span>
        ),
      },
      {
        id: "p95",
        accessorKey: "p95",
        header: "p95 latency",
        cell: ({ row }) => (
          <span className="tabular-nums">
            {row.original.p95LatencyMs !== null ? `${row.original.p95LatencyMs} ms` : "—"}
          </span>
        ),
      },
      {
        id: "incidentCount",
        accessorKey: "incidentCount",
        header: "Incidents",
        cell: ({ row }) => (
          <span
            className={
              row.original.incidentCount > 0
                ? "font-medium text-destructive-foreground tabular-nums"
                : "text-muted-foreground tabular-nums"
            }
          >
            {row.original.incidentCount}
          </span>
        ),
      },
    ],
    [],
  );

  const windowLabel = hours === 1 ? "last hour" : `last ${hours} hours`;

  return (
    <section aria-labelledby="health-title">
      <PageHeader
        title={<span id="health-title">Health</span>}
        description="Probe outcomes and response times the webapi has retained (about a day, in memory)."
        actions={
          <Tabs value={String(hours)} onValueChange={(v) => setHours(Number(v) as Window)}>
            <TabsList aria-label="Time window">
              <TabsTab value="1">1h</TabsTab>
              <TabsTab value="6">6h</TabsTab>
              <TabsTab value="24">24h</TabsTab>
            </TabsList>
          </Tabs>
        }
      />

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatTile
          label={`Platform uptime, ${windowLabel}`}
          value={health.data ? formatUptime(totals.uptime) : undefined}
          detail="Healthy probes over all probes"
          loading={health.isPending}
          tone={totals.uptime !== null && totals.uptime < 99 ? "warning" : "default"}
        />
        <StatTile
          label="Healthy now"
          value={health.data ? `${totals.healthyNow}/${services.length}` : undefined}
          detail="Probed services"
          to="/admin/services"
          loading={health.isPending}
          tone={services.length > 0 && totals.healthyNow < services.length ? "danger" : "default"}
        />
        <StatTile
          label={`Incidents, ${windowLabel}`}
          value={health.data ? totals.incidents.length : undefined}
          detail={
            totals.open > 0 ? `${pluralize(totals.open, "incident")} ongoing` : "none ongoing"
          }
          loading={health.isPending}
          tone={totals.open > 0 ? "danger" : "default"}
        />
        <StatTile
          label="Median latency now"
          value={
            health.data ? (totals.medianNow !== null ? `${totals.medianNow} ms` : "—") : undefined
          }
          detail="Median of each service's p50"
          loading={health.isPending}
        />
      </div>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle>Platform uptime</CardTitle>
          <CardDescription>
            Worst outcome across every probed service per period, {windowLabel}.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {health.isPending ? (
            <Skeleton className="h-6" shape="block" />
          ) : (
            <HealthBucketStrip buckets={platform} />
          )}
        </CardContent>
      </Card>

      <div className="mt-6">
        <HealthPanel
          data={health.data}
          loading={health.isPending}
          error={health.error}
          hours={hours}
        />
      </div>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle>Response time by service</CardTitle>
          <CardDescription>
            95th-percentile probe latency over the {windowLabel}, slowest first.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {health.isPending ? (
            <Skeleton className="h-24" shape="block" />
          ) : services.every((s) => s.p95LatencyMs === null) ? (
            <EmptyState
              title="No latency recorded"
              description="Latency appears once probes have run."
            />
          ) : (
            <LatencyBars
              label={`p95 latency by service, ${windowLabel}`}
              items={services
                .filter((s) => s.p95LatencyMs !== null)
                .map((s) => ({ name: s.displayName, value: s.p95LatencyMs as number }))
                .sort((a, b) => b.value - a.value)}
            />
          )}
        </CardContent>
      </Card>

      <div className="mt-8 flex flex-col gap-3">
        <div>
          <h3 className="text-base font-semibold text-foreground">Scorecard</h3>
          <p className="text-sm text-muted-foreground">
            Per service over the {windowLabel}; sort a column to rank.
          </p>
        </div>
        <DataTable
          ariaLabel="Service health scorecard"
          columns={columns}
          data={rows}
          getRowId={(r) => r.id}
          getRowLabel={(r) => r.displayName}
          loading={health.isPending}
          error={health.error ? health.error.message : undefined}
          onRetry={() => void health.refetch()}
          emptyTitle="No probed services"
          emptyDescription="Enable landingPage.healthCheck on a NebariApp to see it here."
        />
      </div>

      <div className="mt-8 flex flex-col gap-3">
        <div>
          <h3 className="text-base font-semibold text-foreground">Incidents</h3>
          <p className="text-sm text-muted-foreground">
            Contiguous runs of failed probes, newest first.
          </p>
        </div>
        {health.isPending ? (
          <Skeleton className="h-16" shape="block" />
        ) : totals.incidents.length === 0 ? (
          <EmptyState
            title="No incidents"
            description={`No service failed a probe in the ${windowLabel}.`}
          />
        ) : (
          <Table aria-label="Incidents">
            <TableHeader>
              <TableRow>
                <TableHead>Service</TableHead>
                <TableHead>Started</TableHead>
                <TableHead>Duration</TableHead>
                <TableHead>Failed probes</TableHead>
                <TableHead>State</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {totals.incidents.map((i) => (
                <TableRow key={`${i.serviceId}-${i.from}`}>
                  <TableCell>
                    <Link
                      to={`/admin/services/${encodeURIComponent(i.serviceId)}`}
                      className="font-medium hover:underline"
                    >
                      {i.displayName}
                    </Link>
                  </TableCell>
                  <TableCell className="text-muted-foreground">{formatDateTime(i.from)}</TableCell>
                  <TableCell className="tabular-nums">{fmtDuration(i.from, i.to)}</TableCell>
                  <TableCell className="tabular-nums">{i.probes}</TableCell>
                  <TableCell>
                    {i.to === null ? (
                      <StatusBadge status="unhealthy" />
                    ) : (
                      <span className="text-muted-foreground">Resolved {formatDateTime(i.to)}</span>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>
    </section>
  );
}
