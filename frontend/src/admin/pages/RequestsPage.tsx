import { Check, X } from "lucide-react";
import { useMemo, useState } from "react";
import { Link } from "react-router";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumnDef } from "@/components/ui/data-table";
import { Tabs, TabsList, TabsTab } from "@/components/ui/tabs";
import type { AccessRequest } from "../api/types";
import { PageHeader } from "../components/PageHeader";
import { useAccessRequests, useAdminWorld, useResolveAccessRequest } from "../hooks/useAdminData";
import { formatDateTime, pluralize } from "../lib/format";

type Row = AccessRequest & Record<string, unknown> & { search: string };

/**
 * Access-request queue. Approving adds the user to the service's required
 * groups in Keycloak (the webapi does that), so a decision here is the same
 * as editing group membership by hand.
 */
export function RequestsPage() {
  const [view, setView] = useState<"pending" | "resolved">("pending");
  const requests = useAccessRequests();
  const resolve = useResolveAccessRequest();
  const { users, services } = useAdminWorld();

  const rows = useMemo<Row[]>(
    () =>
      (requests.data ?? [])
        .filter((r) => (view === "pending" ? r.status === "pending" : r.status !== "pending"))
        .sort((a, b) =>
          view === "pending"
            ? a.requestedAt.localeCompare(b.requestedAt)
            : b.resolvedAt.localeCompare(a.resolvedAt),
        )
        .map((r) => ({ ...r, search: `${r.userID} ${r.userEmail} ${r.serviceName} ${r.message}` })),
    [requests.data, view],
  );
  const pendingCount = (requests.data ?? []).filter((r) => r.status === "pending").length;
  const userByName = useMemo(() => new Map(users.map((u) => [u.username, u])), [users]);
  const serviceById = useMemo(() => new Map(services.map((s) => [s.id, s])), [services]);

  const columns = useMemo<DataTableColumnDef<Row>[]>(
    () => [
      {
        id: "search",
        accessorKey: "search",
        header: "Requester",
        cell: ({ row }) => {
          const u = userByName.get(row.original.userID);
          return (
            <div className="flex min-w-0 flex-col">
              {u ? (
                <Link
                  to={`/admin/users/${encodeURIComponent(u.id)}`}
                  className="truncate font-medium text-foreground hover:underline"
                >
                  {row.original.userID}
                </Link>
              ) : (
                <span className="truncate font-medium text-foreground">{row.original.userID}</span>
              )}
              <span className="truncate text-xs text-muted-foreground">
                {row.original.userEmail}
              </span>
            </div>
          );
        },
      },
      {
        id: "serviceName",
        accessorKey: "serviceName",
        header: "Service",
        cell: ({ row }) => {
          const s = serviceById.get(row.original.serviceUID);
          return (
            <div className="flex min-w-0 flex-col">
              {s ? (
                <Link
                  to={`/admin/services/${encodeURIComponent(s.id)}`}
                  className="font-medium hover:underline"
                >
                  {row.original.serviceName}
                </Link>
              ) : (
                <span className="font-medium">{row.original.serviceName}</span>
              )}
              {s && s.requiredGroups.length > 0 ? (
                <span className="text-xs text-muted-foreground">
                  Approving joins {s.requiredGroups.join(", ")}
                </span>
              ) : null}
            </div>
          );
        },
      },
      {
        id: "message",
        header: "Message",
        enableSorting: false,
        cell: ({ row }) => (
          <span className="line-clamp-2 text-sm text-muted-foreground">
            {row.original.message || "—"}
          </span>
        ),
      },
      view === "pending"
        ? {
            id: "requestedAt",
            accessorKey: "requestedAt",
            header: "Requested",
            cell: ({ row }) => (
              <span className="text-muted-foreground">
                {formatDateTime(row.original.requestedAt)}
              </span>
            ),
          }
        : {
            id: "resolvedAt",
            accessorKey: "resolvedAt",
            header: "Decision",
            cell: ({ row }) => (
              <div className="flex flex-col gap-1">
                <span>
                  {row.original.status === "approved" ? (
                    <Badge className="bg-success text-success-foreground">Approved</Badge>
                  ) : (
                    <Badge variant="destructive">Denied</Badge>
                  )}
                </span>
                <span className="text-xs text-muted-foreground">
                  by {row.original.resolvedBy || "an admin"} ·{" "}
                  {formatDateTime(row.original.resolvedAt)}
                </span>
              </div>
            ),
          },
      ...(view === "pending"
        ? [
            {
              id: "actions",
              header: "",
              enableSorting: false,
              cell: ({ row }) => (
                <div className="flex justify-end gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    aria-label={`Deny ${row.original.userID} for ${row.original.serviceName}`}
                    loading={resolve.isPending && resolve.variables?.id === row.original.id}
                    onClick={() => resolve.mutate({ id: row.original.id, action: "deny" })}
                  >
                    <X aria-hidden="true" />
                    Deny
                  </Button>
                  <Button
                    size="sm"
                    aria-label={`Approve ${row.original.userID} for ${row.original.serviceName}`}
                    loading={resolve.isPending && resolve.variables?.id === row.original.id}
                    onClick={() => resolve.mutate({ id: row.original.id, action: "approve" })}
                  >
                    <Check aria-hidden="true" />
                    Approve
                  </Button>
                </div>
              ),
            } satisfies DataTableColumnDef<Row>,
          ]
        : []),
    ],
    [view, userByName, serviceById, resolve],
  );

  return (
    <section aria-labelledby="requests-title">
      <PageHeader
        title={<span id="requests-title">Access requests</span>}
        description={
          requests.isPending
            ? "Loading…"
            : `${pluralize(pendingCount, "request")} waiting for a decision. Approving adds the requester to the service's groups.`
        }
      />

      <Tabs
        value={view}
        onValueChange={(v) => setView(v as "pending" | "resolved")}
        className="mb-4"
      >
        <TabsList aria-label="Request status">
          <TabsTab value="pending">Pending ({pendingCount})</TabsTab>
          <TabsTab value="resolved">
            Resolved ({(requests.data ?? []).length - pendingCount})
          </TabsTab>
        </TabsList>
      </Tabs>

      <DataTable
        ariaLabel={view === "pending" ? "Pending access requests" : "Resolved access requests"}
        columns={columns}
        data={rows}
        getRowId={(r) => r.id}
        getRowLabel={(r) => `${r.userID} for ${r.serviceName}`}
        filterColumnId="search"
        filterLabel="Search requests"
        filterPlaceholder="Search requester, service or message…"
        initialPageSize={25}
        showPagination={rows.length > 25}
        loading={requests.isPending}
        error={requests.error ? requests.error.message : undefined}
        onRetry={() => void requests.refetch()}
        emptyTitle={view === "pending" ? "No pending requests" : "No decisions yet"}
        emptyDescription={
          view === "pending"
            ? "Requests appear here when a signed-in user asks for a service they can't reach."
            : "Approved and denied requests will be listed here."
        }
        filteredEmptyTitle="No requests match"
        filteredEmptyDescription="Try a different search."
      />
    </section>
  );
}
