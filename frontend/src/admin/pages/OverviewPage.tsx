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
import { StatusBadge as AccountStatusBadge } from "../components/EntityBadges";
import { PageHeader } from "../components/PageHeader";
import { StatTile } from "../components/StatTile";
import { VersionBadge } from "../components/VersionBadge";
import { useAdminOverview, useAdminWorld, usePacks } from "../hooks/useAdminData";
import { displayName, formatDateTime, pluralize } from "../lib/format";

type AttentionRow = {
  key: string;
  kind: "service" | "pack" | "platform" | "accounts";
  item: React.ReactNode;
  why: React.ReactNode;
  to: string;
  action: string;
};

/**
 * Admin landing page: a KPI row from /admin/overview, then a full-width
 * "needs attention" table derived from the entity queries. The activity
 * feed lives in its own section.
 */
export function OverviewPage() {
  const overview = useAdminOverview();
  const packs = usePacks();
  const { users, services, isLoading: worldLoading } = useAdminWorld();
  const o = overview.data;
  const packList = packs.data?.packs ?? [];

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
      const behind = p.versionStatus === "behind";
      if (!bad && !drift && !behind) continue;
      rows.push({
        key: `pack-${p.name}`,
        kind: p.tier,
        item: p.name,
        why: (
          <span className="flex flex-wrap items-center gap-1">
            {drift ? <SyncBadge status={p.argo.syncStatus} /> : null}
            {bad ? <ArgoHealthBadge status={p.argo.healthStatus} /> : null}
            {behind ? <VersionBadge pack={p} /> : null}
            <span className="text-muted-foreground">
              {p.chartName}
              {p.chartVersion ? ` ${p.chartVersion}` : ""}
            </span>
          </span>
        ),
        to: `/admin/packs/${encodeURIComponent(p.name)}`,
        action: p.tier === "pack" ? "Open pack" : "Open component",
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
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
          <StatTile
            label="Users"
            value={o?.users.total}
            detail={
              o ? `${o.users.disabled} disabled · ${o.users.createdLast7Days} new this week` : null
            }
            to="/admin/users"
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
          <StatTile
            label="Services"
            value={o ? `${o.services.healthy}/${o.services.total}` : undefined}
            detail={
              o
                ? `healthy · ${o.services.unhealthy} unhealthy · ${o.services.unknown} unknown`
                : null
            }
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
        </div>
      )}

      <div className="mt-8 flex flex-col gap-3">
        <div>
          <h3 className="text-base font-semibold text-foreground">Needs attention</h3>
          <p className="text-sm text-muted-foreground">
            Unhealthy services, drifted, degraded or outdated packs, and accounts that can't reach
            anything.
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
    </section>
  );
}
