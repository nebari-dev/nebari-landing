import { ArrowRight } from "lucide-react";
import { useMemo } from "react";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "../components/EmptyState";
import { GroupBadge, StatusBadge } from "../components/EntityBadges";
import { PageHeader } from "../components/PageHeader";
import { StatTile } from "../components/StatTile";
import { useAccessRequests, useAdminOverview, useAdminWorld } from "../hooks/useAdminData";
import { displayName, formatDateTime, pluralize } from "../lib/format";

/**
 * Admin landing page: a KPI row from /admin/overview, then two lists derived
 * from the entity queries: what needs attention and what changed recently.
 */
export function OverviewPage() {
  const overview = useAdminOverview();
  const pending = useAccessRequests("pending");
  const resolved = useAccessRequests();
  const { users, groups, isLoading: worldLoading } = useAdminWorld();
  const o = overview.data;

  const usersWithoutGroups = useMemo(() => users.filter((u) => u.groups.length === 0), [users]);
  const recentUsers = useMemo(
    () =>
      [...users]
        .filter((u) => u.createdAt)
        .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
        .slice(0, 5),
    [users],
  );
  const recentlyResolved = useMemo(
    () =>
      (resolved.data ?? [])
        .filter((r) => r.status !== "pending" && r.resolvedAt)
        .sort((a, b) => b.resolvedAt.localeCompare(a.resolvedAt))
        .slice(0, 5),
    [resolved.data],
  );
  const groupName = useMemo(() => new Map(groups.map((g) => [g.id, g.name])), [groups]);
  const loading = overview.isPending;

  return (
    <section aria-labelledby="overview-title">
      <PageHeader
        title={<span id="overview-title">Overview</span>}
        description={
          o?.generatedAt
            ? `Figures as of ${formatDateTime(o.generatedAt)}. Refreshes automatically.`
            : "Headline figures for the platform."
        }
      />

      {overview.error ? (
        <EmptyState
          variant="error"
          title="Couldn't load overview figures"
          description={overview.error.message}
          action={
            <Button variant="outline" onClick={() => void overview.refetch()}>
              Retry
            </Button>
          }
        />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
          <StatTile
            label="Accounts"
            value={o?.users.total}
            detail={
              o ? `${o.users.disabled} disabled · ${o.users.createdLast7Days} new this week` : null
            }
            to="/admin/users"
            loading={loading}
          />
          <StatTile
            label="Online now"
            value={o?.activeSessions ?? (loading ? undefined : "—")}
            detail="Active Keycloak sessions"
            loading={loading}
          />
          <StatTile
            label="Pending requests"
            value={o?.accessRequests.pending}
            detail={
              o ? `${o.accessRequests.approved} approved · ${o.accessRequests.denied} denied` : null
            }
            to="/admin/requests"
            tone={o && o.accessRequests.pending > 0 ? "warning" : "default"}
            loading={loading}
          />
          <StatTile
            label="Services healthy"
            value={o ? `${o.services.healthy}/${o.services.total}` : undefined}
            detail={o ? `${o.services.unhealthy} unhealthy · ${o.services.unknown} unknown` : null}
            to="/admin/services"
            tone={o && o.services.unhealthy > 0 ? "danger" : "default"}
            loading={loading}
          />
          <StatTile
            label="Groups"
            value={o?.groups}
            detail={
              o
                ? `${o.services.gated} gated ${o.services.gated === 1 ? "service" : "services"}`
                : null
            }
            to="/admin/groups"
            loading={loading}
          />
          <StatTile
            label="Roles"
            value={o?.roles}
            detail="Custom realm roles"
            to="/admin/roles"
            loading={loading}
          />
        </div>
      )}

      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Needs attention</CardTitle>
            <CardDescription>Open decisions and gaps an admin should look at.</CardDescription>
          </CardHeader>
          <CardContent>
            {worldLoading || pending.isPending ? (
              <p className="text-sm text-muted-foreground">Loading…</p>
            ) : (pending.data?.length ?? 0) === 0 &&
              usersWithoutGroups.length === 0 &&
              (o?.users.disabled ?? 0) === 0 &&
              (o?.services.unhealthy ?? 0) === 0 ? (
              <EmptyState
                title="Nothing waiting on you"
                description="No pending requests, every account is in a group, and no service is unhealthy."
              />
            ) : (
              <ul className="divide-y divide-border">
                {(pending.data ?? []).slice(0, 5).map((r) => (
                  <li key={r.id} className="flex items-center justify-between gap-3 py-2">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium text-foreground">
                        {r.userID} wants {r.serviceName}
                      </p>
                      <p className="truncate text-xs text-muted-foreground">
                        {r.message || "No message"} · {formatDateTime(r.requestedAt)}
                      </p>
                    </div>
                    <Button size="sm" variant="outline" render={<Link to="/admin/requests" />}>
                      Review
                      <ArrowRight aria-hidden="true" />
                    </Button>
                  </li>
                ))}
                {usersWithoutGroups.length > 0 ? (
                  <li className="flex items-center justify-between gap-3 py-2">
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-foreground">
                        {pluralize(usersWithoutGroups.length, "account")} in no group
                      </p>
                      <p className="truncate text-xs text-muted-foreground">
                        {usersWithoutGroups
                          .slice(0, 3)
                          .map((u) => displayName(u))
                          .join(", ")}
                        {usersWithoutGroups.length > 3 ? "…" : ""}
                      </p>
                    </div>
                    <Button size="sm" variant="outline" render={<Link to="/admin/users" />}>
                      Open users
                      <ArrowRight aria-hidden="true" />
                    </Button>
                  </li>
                ) : null}
                {o && o.users.disabled > 0 ? (
                  <li className="flex items-center justify-between gap-3 py-2">
                    <p className="text-sm font-medium text-foreground">
                      {pluralize(o.users.disabled, "disabled account")}
                    </p>
                    <StatusBadge enabled={false} />
                  </li>
                ) : null}
              </ul>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Recent activity</CardTitle>
            <CardDescription>Newest accounts and the latest access decisions.</CardDescription>
          </CardHeader>
          <CardContent>
            {worldLoading ? (
              <p className="text-sm text-muted-foreground">Loading…</p>
            ) : recentUsers.length === 0 && recentlyResolved.length === 0 ? (
              <EmptyState title="No activity yet" />
            ) : (
              <ul className="divide-y divide-border">
                {recentlyResolved.map((r) => (
                  <li key={r.id} className="py-2">
                    <p className="text-sm text-foreground">
                      <span className="font-medium">{r.resolvedBy || "An admin"}</span>{" "}
                      {r.status === "approved" ? "approved" : "denied"}{" "}
                      <span className="font-medium">{r.userID}</span> for {r.serviceName}
                    </p>
                    <p className="text-xs text-muted-foreground">{formatDateTime(r.resolvedAt)}</p>
                  </li>
                ))}
                {recentUsers.map((u) => (
                  <li key={u.id} className="flex items-center justify-between gap-3 py-2">
                    <div className="min-w-0">
                      <p className="truncate text-sm text-foreground">
                        <Link
                          to={`/admin/users/${encodeURIComponent(u.id)}`}
                          className="font-medium hover:underline"
                        >
                          {displayName(u)}
                        </Link>{" "}
                        joined
                      </p>
                      <p className="text-xs text-muted-foreground">{formatDateTime(u.createdAt)}</p>
                    </div>
                    <span className="flex flex-wrap justify-end gap-1">
                      {u.groups.slice(0, 2).map((gid) => (
                        <GroupBadge key={gid} id={gid} name={groupName.get(gid) ?? gid} />
                      ))}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>
    </section>
  );
}
