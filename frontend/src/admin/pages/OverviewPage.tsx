import { ArrowRight } from "lucide-react";
import { useMemo } from "react";
import { Link } from "react-router";
import { StatusBadge } from "@/components/StatusBadge";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ArgoHealthBadge, SyncBadge } from "../components/ArgoBadges";
import { EmptyState } from "../components/EmptyState";
import { StatusBadge as AccountStatusBadge, GroupBadge } from "../components/EntityBadges";
import { PageHeader } from "../components/PageHeader";
import { StatTile } from "../components/StatTile";
import { useAdminOverview, useAdminWorld, usePacks } from "../hooks/useAdminData";
import { displayName, formatDateTime, pluralize } from "../lib/format";

type AttentionRow = {
  key: string;
  kind: "service" | "pack" | "accounts";
  item: React.ReactNode;
  why: React.ReactNode;
  to: string;
  action: string;
};

type ActivityRow = {
  key: string;
  at: string;
  event: string;
  subject: React.ReactNode;
  details: React.ReactNode;
};

/**
 * Admin landing page: a KPI row from /admin/overview, then two full-width
 * tables derived from the entity queries: what needs attention and what
 * changed recently.
 */
export function OverviewPage() {
  const overview = useAdminOverview();
  const packs = usePacks();
  const { users, groups, services, isLoading: worldLoading } = useAdminWorld();
  const o = overview.data;
  const packList = packs.data?.packs ?? [];
  const groupName = useMemo(() => new Map(groups.map((g) => [g.id, g.name])), [groups]);

  const attention = useMemo<AttentionRow[]>(() => {
    const rows: AttentionRow[] = [];
    for (const s of services) {
      if (s.health?.status === "unhealthy") {
        rows.push({
          key: `svc-${s.id}`,
          kind: "service",
          item: s.displayName,
          why: (
            <span className="flex flex-wrap items-center gap-2">
              <StatusBadge status="unhealthy" />
              <span>{s.health.message || "Health probe failing"}</span>
            </span>
          ),
          to: `/admin/services/${encodeURIComponent(s.id)}`,
          action: "Open service",
        });
      }
    }
    for (const p of packList) {
      if (!p.argo) continue;
      const bad = p.argo.healthStatus !== "Healthy" && p.argo.healthStatus !== "Unknown";
      const drift = p.argo.syncStatus === "OutOfSync";
      if (!bad && !drift) continue;
      rows.push({
        key: `pack-${p.name}`,
        kind: "pack",
        item: p.name,
        why: (
          <span className="flex flex-wrap items-center gap-1">
            {drift ? <SyncBadge status={p.argo.syncStatus} /> : null}
            {bad ? <ArgoHealthBadge status={p.argo.healthStatus} /> : null}
            <span className="text-muted-foreground">
              {p.chartName}
              {p.chartVersion ? ` ${p.chartVersion}` : ""}
            </span>
          </span>
        ),
        to: `/admin/packs/${encodeURIComponent(p.name)}`,
        action: "Open pack",
      });
    }
    const noGroups = users.filter((u) => u.groups.length === 0);
    if (noGroups.length > 0) {
      rows.push({
        key: "no-groups",
        kind: "accounts",
        item: `${pluralize(noGroups.length, "account")} in no group`,
        why: `${noGroups
          .slice(0, 3)
          .map((u) => displayName(u))
          .join(", ")}${noGroups.length > 3 ? "…" : ""} can reach only public services`,
        to: "/admin/users",
        action: "Open users",
      });
    }
    const disabled = users.filter((u) => !u.enabled);
    if (disabled.length > 0) {
      rows.push({
        key: "disabled",
        kind: "accounts",
        item: `${pluralize(disabled.length, "disabled account")}`,
        why: (
          <span className="flex flex-wrap items-center gap-2">
            <AccountStatusBadge enabled={false} />
            <span>
              {disabled
                .slice(0, 3)
                .map((u) => displayName(u))
                .join(", ")}
              {disabled.length > 3 ? "…" : ""}
            </span>
          </span>
        ),
        to: "/admin/users",
        action: "Open users",
      });
    }
    return rows;
  }, [services, packList, users]);

  const activity = useMemo<ActivityRow[]>(() => {
    const rows: ActivityRow[] = [];
    for (const u of users) {
      if (!u.createdAt) continue;
      rows.push({
        key: `user-${u.id}`,
        at: u.createdAt,
        event: "Account created",
        subject: (
          <Link
            to={`/admin/users/${encodeURIComponent(u.id)}`}
            className="font-medium hover:underline"
          >
            {displayName(u)}
          </Link>
        ),
        details: (
          <span className="flex flex-wrap gap-1">
            {u.groups.slice(0, 3).map((gid) => (
              <GroupBadge key={gid} id={gid} name={groupName.get(gid) ?? gid} />
            ))}
            {u.groups.length === 0 ? (
              <span className="text-muted-foreground">no groups</span>
            ) : null}
          </span>
        ),
      });
    }
    for (const p of packList) {
      if (!p.argo?.lastSyncAt) continue;
      rows.push({
        key: `sync-${p.name}`,
        at: p.argo.lastSyncAt,
        event: `Pack sync ${p.argo.lastSyncPhase?.toLowerCase() || "finished"}`,
        subject: (
          <Link
            to={`/admin/packs/${encodeURIComponent(p.name)}`}
            className="font-medium hover:underline"
          >
            {p.name}
          </Link>
        ),
        details: (
          <span className="text-muted-foreground">
            {p.chartName}
            {p.chartVersion ? ` ${p.chartVersion}` : ""}
            {p.argo.revision ? ` · ${p.argo.revision.slice(0, 7)}` : ""}
          </span>
        ),
      });
    }
    return rows.sort((a, b) => b.at.localeCompare(a.at)).slice(0, 10);
  }, [users, packList, groupName]);

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
            label="Services healthy"
            value={o ? `${o.services.healthy}/${o.services.total}` : undefined}
            detail={o ? `${o.services.unhealthy} unhealthy · ${o.services.unknown} unknown` : null}
            to="/admin/services"
            tone={o && o.services.unhealthy > 0 ? "danger" : "default"}
            loading={loading}
          />
          <StatTile
            label="Packs"
            value={packs.data ? packList.filter((p) => p.tier === "pack").length : undefined}
            detail={
              packs.data
                ? `${packList.filter((p) => p.argo?.syncStatus === "OutOfSync").length} out of sync`
                : null
            }
            to="/admin/packs"
            tone={
              packList.some((p) => p.argo && p.argo.healthStatus === "Degraded")
                ? "danger"
                : "default"
            }
            loading={packs.isPending}
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

      <div className="mt-8 flex flex-col gap-3">
        <div>
          <h3 className="text-base font-semibold text-foreground">Needs attention</h3>
          <p className="text-sm text-muted-foreground">
            Unhealthy services, drifted or degraded packs, and accounts that can't reach anything.
          </p>
        </div>
        {worldLoading || packs.isPending ? (
          <p className="text-sm text-muted-foreground">Loading…</p>
        ) : attention.length === 0 ? (
          <EmptyState
            title="Nothing waiting on you"
            description="Every service is healthy, every pack is in sync, and every account is in a group."
          />
        ) : (
          <Table aria-label="Needs attention">
            <TableHeader>
              <TableRow>
                <TableHead className="w-40">Type</TableHead>
                <TableHead>Item</TableHead>
                <TableHead>Why</TableHead>
                <TableHead className="w-36" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {attention.map((r) => (
                <TableRow key={r.key}>
                  <TableCell>
                    <Badge variant="ghost" className="text-muted-foreground">
                      {r.kind}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-medium text-foreground">{r.item}</TableCell>
                  <TableCell>{r.why}</TableCell>
                  <TableCell className="text-right">
                    <Button size="sm" variant="outline" render={<Link to={r.to} />}>
                      {r.action}
                      <ArrowRight aria-hidden="true" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      <div className="mt-8 flex flex-col gap-3">
        <div>
          <h3 className="text-base font-semibold text-foreground">Recent activity</h3>
          <p className="text-sm text-muted-foreground">
            Newest accounts and the latest pack syncs.
          </p>
        </div>
        {worldLoading || packs.isPending ? (
          <p className="text-sm text-muted-foreground">Loading…</p>
        ) : activity.length === 0 ? (
          <EmptyState title="No activity yet" />
        ) : (
          <Table aria-label="Recent activity">
            <TableHeader>
              <TableRow>
                <TableHead className="w-48">When</TableHead>
                <TableHead className="w-48">Event</TableHead>
                <TableHead>Subject</TableHead>
                <TableHead>Details</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {activity.map((r) => (
                <TableRow key={r.key}>
                  <TableCell className="text-muted-foreground">{formatDateTime(r.at)}</TableCell>
                  <TableCell>{r.event}</TableCell>
                  <TableCell>{r.subject}</TableCell>
                  <TableCell>{r.details}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>
    </section>
  );
}
