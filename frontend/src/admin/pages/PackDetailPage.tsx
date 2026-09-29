import { ExternalLink, Package } from "lucide-react";
import { Link, useParams } from "react-router";
import { StatusBadge } from "@/components/StatusBadge";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
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
import { GroupBadge, VisibilityBadge } from "../components/EntityBadges";
import { PageHeader } from "../components/PageHeader";
import { StatTile } from "../components/StatTile";
import { useAdminWorld, usePacks } from "../hooks/useAdminData";
import { formatDateTime, pluralize } from "../lib/format";

export function PackDetailPage() {
  const { name = "" } = useParams();
  const packs = usePacks();
  const { groups } = useAdminWorld();
  const pack = packs.data?.packs.find((p) => p.name === name);

  if (packs.isPending) {
    return (
      <output aria-label="Loading pack" className="flex flex-col gap-3">
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-8 w-72" />
        <Skeleton className="h-64" shape="block" />
      </output>
    );
  }
  if (!pack) {
    return (
      <EmptyState
        title="Pack not found"
        action={
          <Button variant="outline" render={<Link to="/admin/packs" />}>
            Back to packs
          </Button>
        }
      />
    );
  }

  const argo = pack.argo;
  const source = argo
    ? `${argo.repoURL}${argo.chart ? ` · chart ${argo.chart}` : argo.path ? ` · ${argo.path}` : ""} @ ${argo.targetRevision}`
    : null;
  const unhealthy = pack.services.filter((s) => s.healthStatus === "unhealthy").length;

  return (
    <section aria-labelledby="pack-title">
      <PageHeader
        crumbs={[{ label: "Software packs", to: "/admin/packs" }, { label: pack.name }]}
        leading={<Package aria-hidden="true" className="size-8 text-muted-foreground" />}
        title={
          <span id="pack-title" className="flex items-center gap-2">
            {pack.name}
            {argo ? <SyncBadge status={argo.syncStatus} /> : null}
            {argo ? <ArgoHealthBadge status={argo.healthStatus} /> : null}
            {pack.tier === "platform" ? <Badge variant="ghost">platform</Badge> : null}
          </span>
        }
        description={
          <span className="flex flex-wrap gap-x-3">
            <span className="font-mono text-xs">{pack.namespace}</span>
            {pack.chartName ? (
              <span>
                {pack.chartName}
                {pack.chartVersion ? ` ${pack.chartVersion}` : ""}
              </span>
            ) : null}
            {pack.appVersion ? <span>app {pack.appVersion}</span> : null}
          </span>
        }
      />

      <div className="mb-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatTile
          label="Chart version"
          value={pack.chartVersion || argo?.targetRevision || "—"}
          detail={pack.chartName}
        />
        <StatTile
          label="Last sync"
          value={argo?.lastSyncAt ? formatDateTime(argo.lastSyncAt) : "—"}
          detail={
            argo
              ? `${argo.lastSyncPhase || "unknown"} · ${argo.autoSync ? "auto-sync on" : "manual sync"}`
              : "ArgoCD not readable"
          }
        />
        <StatTile
          label="Services"
          value={pack.services.length}
          detail={
            unhealthy > 0
              ? `${pluralize(unhealthy, "service")} unhealthy`
              : "all probes healthy or unknown"
          }
          tone={unhealthy > 0 ? "danger" : "default"}
        />
        <StatTile
          label="Resources"
          value={argo?.resourceCount ?? "—"}
          detail={argo ? `${pluralize(argo.images.length, "image")} deployed` : undefined}
        />
      </div>

      <div className="grid gap-6 lg:grid-cols-[2fr_1fr]">
        <Card>
          <CardHeader>
            <CardTitle>Services this pack publishes</CardTitle>
            <CardDescription>
              Each is a NebariApp the pack ships; the gate is declared there. Internal pack
              permissions are managed inside the pack.
            </CardDescription>
          </CardHeader>
          <CardContent>
            {pack.services.length === 0 ? (
              <EmptyState
                title="No landing-page services"
                description="This pack ships no NebariApp with landingPage.enabled."
              />
            ) : (
              <Table aria-label={`Services of ${pack.name}`}>
                <TableHeader>
                  <TableRow>
                    <TableHead>Service</TableHead>
                    <TableHead>Health</TableHead>
                    <TableHead>Gate</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {pack.services.map((s) => (
                    <TableRow key={s.id}>
                      <TableCell>
                        <Link
                          to={`/admin/services/${encodeURIComponent(s.id)}`}
                          className="font-medium hover:underline"
                        >
                          {s.displayName}
                        </Link>
                        <span className="block text-xs text-muted-foreground">
                          {s.namespace}/{s.name}
                        </span>
                      </TableCell>
                      <TableCell>
                        <StatusBadge status={s.healthStatus} />
                      </TableCell>
                      <TableCell>
                        <span className="flex flex-wrap items-center gap-1">
                          <VisibilityBadge visibility={s.visibility} />
                          {s.requiredGroups.map((g) => (
                            <GroupBadge
                              key={g}
                              id={groups.find((x) => x.name === g)?.id ?? null}
                              name={g}
                            />
                          ))}
                        </span>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>

        <div className="flex flex-col gap-6">
          <Card>
            <CardHeader>
              <CardTitle>Source</CardTitle>
              <CardDescription>Where ArgoCD deploys this pack from.</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-2 text-sm">
              {argo ? (
                <>
                  <p className="break-all font-mono text-xs">{source}</p>
                  {argo.revision ? (
                    <p className="text-xs text-muted-foreground">
                      Deployed revision{" "}
                      <span className="font-mono">{argo.revision.slice(0, 12)}</span>
                    </p>
                  ) : null}
                  <p className="text-xs text-muted-foreground">
                    Reconciled {argo.reconciledAt ? formatDateTime(argo.reconciledAt) : "—"}
                  </p>
                  {argo.repoURL.startsWith("https://") ? (
                    <a
                      href={argo.repoURL}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 text-primary hover:underline"
                    >
                      Open repository
                      <ExternalLink aria-hidden="true" className="size-3.5" />
                    </a>
                  ) : null}
                </>
              ) : (
                <p className="text-muted-foreground">
                  ArgoCD Applications are not readable from the webapi; chart details come from the
                  NebariApp Helm labels.
                </p>
              )}
            </CardContent>
          </Card>

          {argo && argo.images.length > 0 ? (
            <Card>
              <CardHeader>
                <CardTitle>Images</CardTitle>
                <CardDescription>
                  {pluralize(argo.images.length, "container image")} in the deployed revision.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <ul className="flex flex-col gap-1">
                  {argo.images.map((img) => (
                    <li key={img} className="break-all font-mono text-xs">
                      {img}
                    </li>
                  ))}
                </ul>
              </CardContent>
            </Card>
          ) : null}
        </div>
      </div>
    </section>
  );
}
