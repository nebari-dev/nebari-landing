import { useMemo, useState } from "react";
import { Link } from "react-router";
import { DataTable, type DataTableColumnDef } from "@/components/ui/data-table";
import { Tabs, TabsList, TabsTab } from "@/components/ui/tabs";
import { GroupBadge } from "../components/EntityBadges";
import { PageHeader } from "../components/PageHeader";
import { useAdminWorld, usePacks } from "../hooks/useAdminData";
import type { ActivityEvent, ActivityKind } from "../lib/activity";
import { buildActivity } from "../lib/activity";
import { formatDateTime, pluralize } from "../lib/format";

type Row = ActivityEvent & Record<string, unknown> & { search: string };

/**
 * Activity feed derived from live objects (see lib/activity.ts). Becomes a
 * real audit log once the webapi records admin actions.
 */
export function ActivityPage() {
  const { users, groups, isLoading: worldLoading, error, refetch } = useAdminWorld();
  const packs = usePacks();
  const [kind, setKind] = useState<"all" | ActivityKind>("all");
  const groupId = useMemo(() => new Map(groups.map((g) => [g.name, g.id])), [groups]);

  const rows = useMemo<Row[]>(
    () =>
      buildActivity(users, groups, packs.data?.packs ?? [])
        .filter((e) => kind === "all" || e.kind === kind)
        .map((e) => ({
          ...e,
          search: `${e.event} ${e.subject} ${e.details} ${e.groups.join(" ")}`,
        })),
    [users, groups, packs.data, kind],
  );

  const columns = useMemo<DataTableColumnDef<Row>[]>(
    () => [
      {
        id: "at",
        accessorKey: "at",
        header: "When",
        cell: ({ row }) => (
          <span className="whitespace-nowrap text-muted-foreground">
            {formatDateTime(row.original.at)}
          </span>
        ),
      },
      { id: "event", accessorKey: "event", header: "Event" },
      {
        id: "search",
        accessorKey: "search",
        header: "Subject",
        cell: ({ row }) => (
          <Link to={row.original.to} className="font-medium text-foreground hover:underline">
            {row.original.subject}
          </Link>
        ),
      },
      {
        id: "details",
        header: "Details",
        enableSorting: false,
        cell: ({ row }) =>
          row.original.groups.length > 0 ? (
            <span className="flex flex-wrap gap-1">
              {row.original.groups.map((g) => (
                <GroupBadge key={g} id={groupId.get(g) ?? null} name={g} />
              ))}
            </span>
          ) : (
            <span className="text-muted-foreground">{row.original.details || "—"}</span>
          ),
      },
    ],
    [groupId],
  );

  const loading = worldLoading || packs.isPending;

  return (
    <section aria-labelledby="activity-title">
      <PageHeader
        title={<span id="activity-title">Activity</span>}
        description={
          loading
            ? "Loading…"
            : `${pluralize(rows.length, "event")}. Derived from account creation and pack syncs; admin actions and sign-ins will appear once the webapi records them.`
        }
      />

      <Tabs value={kind} onValueChange={(v) => setKind(v as "all" | ActivityKind)} className="mb-4">
        <TabsList aria-label="Event type">
          <TabsTab value="all">All</TabsTab>
          <TabsTab value="account">Accounts</TabsTab>
          <TabsTab value="sync">Pack syncs</TabsTab>
        </TabsList>
      </Tabs>

      <DataTable
        ariaLabel="Recent activity"
        columns={columns}
        data={rows}
        getRowId={(r) => r.key}
        getRowLabel={(r) => `${r.event} ${r.subject}`}
        filterColumnId="search"
        filterLabel="Search activity"
        filterPlaceholder="Search events…"
        initialPageSize={25}
        showPagination
        loading={loading}
        error={error ? error.message : undefined}
        onRetry={() => void refetch()}
        emptyTitle="No activity yet"
        emptyDescription="Events appear as accounts are created and packs sync."
        filteredEmptyTitle="No events match"
        filteredEmptyDescription="Try a different search or event type."
      />
    </section>
  );
}
