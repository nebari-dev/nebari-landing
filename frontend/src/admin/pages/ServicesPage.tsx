import { Info } from "lucide-react";
import { useMemo } from "react";
import { Link } from "react-router";
import { StatusBadge } from "@/components/StatusBadge";
import { DataTable, type DataTableColumnDef } from "@/components/ui/data-table";
import type { AdminService } from "../api/types";
import { BadgeOverflow, GroupBadge, VisibilityBadge } from "../components/EntityBadges";
import { formatUptime } from "../components/HealthStrip";
import { PageHeader } from "../components/PageHeader";
import { useAdminWorld } from "../hooks/useAdminData";
import { effectiveUserCount } from "../lib/access";

type Row = AdminService &
  Record<string, unknown> & { reach: number | "everyone"; uptime: number | null };

export function ServicesPage() {
  const { users, groups, services, isLoading, error, refetch } = useAdminWorld();
  const groupId = useMemo(() => new Map(groups.map((g) => [g.name, g.id])), [groups]);

  const rows = useMemo<Row[]>(
    () =>
      services.map((s) => ({
        ...s,
        reach: effectiveUserCount(s, groups, users),
        uptime: s.health?.uptimePercent ?? null,
      })),
    [services, groups, users],
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
            className="flex min-w-0 flex-col rounded-sm outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
          >
            <span className="font-medium text-foreground">{row.original.displayName}</span>
            <span className="truncate text-xs text-muted-foreground">
              {row.original.category} · {row.original.namespace}/{row.original.name}
            </span>
          </Link>
        ),
      },
      {
        id: "visibility",
        accessorKey: "visibility",
        header: "Visibility",
        cell: ({ row }) => <VisibilityBadge visibility={row.original.visibility} />,
      },
      {
        id: "requiredGroups",
        header: "Required groups",
        enableSorting: false,
        cell: ({ row }) =>
          row.original.visibility === "public" ? (
            <span className="text-xs text-muted-foreground">Everyone</span>
          ) : row.original.requiredGroups.length === 0 ? (
            <span className="text-xs text-muted-foreground">Any signed-in user</span>
          ) : (
            <BadgeOverflow
              items={row.original.requiredGroups}
              render={(name) => (
                <GroupBadge key={name} id={groupId.get(name) ?? null} name={name} />
              )}
              getLabel={(name) => name}
            />
          ),
      },
      {
        id: "uptime",
        accessorKey: "uptime",
        header: "Health",
        cell: ({ row }) =>
          row.original.health ? (
            <div className="flex flex-col items-start gap-1">
              <StatusBadge status={row.original.health.status} />
              <span className="text-xs text-muted-foreground tabular-nums">
                {formatUptime(row.original.health.uptimePercent)} uptime
                {row.original.health.samples > 0 ? ` · ${row.original.health.samples} probes` : ""}
              </span>
            </div>
          ) : (
            <span className="text-xs text-muted-foreground">No health check</span>
          ),
      },
      {
        id: "reach",
        accessorKey: "reach",
        header: "Users with access",
        cell: ({ row }) => (
          <span className="tabular-nums">
            {row.original.reach === "everyone" ? "Everyone" : row.original.reach}
          </span>
        ),
      },
    ],
    [groupId],
  );

  return (
    <section aria-labelledby="services-title">
      <PageHeader
        title={<span id="services-title">Service access</span>}
        description={
          <span className="flex items-start gap-1.5">
            <Info aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
            <span>
              Each service's gate comes from its NebariApp and is managed through GitOps. Grant or
              revoke access by changing who is in the required groups.
            </span>
          </span>
        }
      />

      <DataTable
        ariaLabel="Services"
        columns={columns}
        data={rows}
        getRowId={(r) => r.id}
        getRowLabel={(r) => r.displayName}
        filterColumnId="displayName"
        filterLabel="Search services"
        filterPlaceholder="Search services…"
        initialPageSize={25}
        showPagination={rows.length > 25}
        loading={isLoading}
        error={error ? error.message : undefined}
        onRetry={() => void refetch()}
        emptyTitle="No services on the landing page"
        emptyDescription="Services appear once a NebariApp with landingPage.enabled is reconciled."
        filteredEmptyTitle="No services match"
        filteredEmptyDescription="Try a different search."
      />
    </section>
  );
}
