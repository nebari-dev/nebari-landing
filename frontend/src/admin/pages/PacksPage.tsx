import { Info } from "lucide-react";
import { useMemo, useState } from "react";
import { Link } from "react-router";
import { Badge } from "@/components/ui/badge";
import { DataTable, type DataTableColumnDef } from "@/components/ui/data-table";
import { Tabs, TabsList, TabsTab } from "@/components/ui/tabs";
import type { Pack } from "../api/types";
import { ArgoHealthBadge, SyncBadge } from "../components/ArgoBadges";
import { PageHeader } from "../components/PageHeader";
import { usePacks } from "../hooks/useAdminData";
import { formatDateTime, pluralize } from "../lib/format";

type Row = Pack & Record<string, unknown> & { search: string; version: string; lastSync: string };

/**
 * Software packs: one row per ArgoCD Application that ships services, with
 * the version it runs, its sync/health, and the landing-page services it
 * owns. Platform components (NIC foundational apps) sit behind a toggle.
 */
export function PacksPage() {
  const packs = usePacks();
  const [tier, setTier] = useState<"pack" | "platform">("pack");

  const all = packs.data?.packs ?? [];
  const rows = useMemo<Row[]>(
    () =>
      all
        .filter((p) => p.tier === tier)
        .map((p) => ({
          ...p,
          search: `${p.name} ${p.chartName ?? ""} ${p.namespace} ${p.services.map((s) => s.displayName).join(" ")}`,
          version: p.chartVersion ?? p.argo?.targetRevision ?? "",
          lastSync: p.argo?.lastSyncAt ?? "",
        })),
    [all, tier],
  );
  const counts = {
    pack: all.filter((p) => p.tier === "pack").length,
    platform: all.filter((p) => p.tier === "platform").length,
  };

  const columns = useMemo<DataTableColumnDef<Row>[]>(
    () => [
      {
        id: "search",
        accessorKey: "search",
        header: "Pack",
        cell: ({ row }) => (
          <Link
            to={`/admin/packs/${encodeURIComponent(row.original.name)}`}
            className="flex min-w-0 flex-col rounded-sm outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
          >
            <span className="font-medium text-foreground">{row.original.name}</span>
            <span className="truncate text-xs text-muted-foreground">
              {row.original.chartName || row.original.argo?.path || "—"} · {row.original.namespace}
            </span>
          </Link>
        ),
      },
      {
        id: "version",
        accessorKey: "version",
        header: "Version",
        cell: ({ row }) => (
          <div className="flex flex-col">
            <span className="font-mono text-xs">{row.original.version || "—"}</span>
            {row.original.appVersion ? (
              <span className="text-xs text-muted-foreground">app {row.original.appVersion}</span>
            ) : null}
          </div>
        ),
      },
      {
        id: "status",
        header: "Status",
        enableSorting: false,
        cell: ({ row }) =>
          row.original.argo ? (
            <span className="flex flex-wrap gap-1">
              <SyncBadge status={row.original.argo.syncStatus} />
              <ArgoHealthBadge status={row.original.argo.healthStatus} />
            </span>
          ) : (
            <span className="text-xs text-muted-foreground">No ArgoCD data</span>
          ),
      },
      {
        id: "services",
        header: "Services",
        enableSorting: false,
        cell: ({ row }) =>
          row.original.services.length === 0 ? (
            <span className="text-xs text-muted-foreground">None on the landing page</span>
          ) : (
            <span className="flex flex-wrap gap-1">
              {row.original.services.map((s) => (
                <Badge
                  key={s.id}
                  variant="outline"
                  render={<Link to={`/admin/services/${encodeURIComponent(s.id)}`} />}
                >
                  {s.displayName}
                </Badge>
              ))}
            </span>
          ),
      },
      {
        id: "lastSync",
        accessorKey: "lastSync",
        header: "Last sync",
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {row.original.argo?.lastSyncAt ? formatDateTime(row.original.argo.lastSyncAt) : "—"}
          </span>
        ),
      },
    ],
    [],
  );

  return (
    <section aria-labelledby="packs-title">
      <PageHeader
        title={<span id="packs-title">Software packs</span>}
        description={
          <span className="flex items-start gap-1.5">
            <Info aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
            <span>
              What is installed, at which version, and whether ArgoCD has it in sync. Packs own
              their internal permissions; the Launchpad only sees the services they publish.
              {packs.data && !packs.data.argocdAvailable
                ? " ArgoCD is not readable from the webapi, so sync and health are unavailable."
                : ""}
            </span>
          </span>
        }
      />

      <Tabs value={tier} onValueChange={(v) => setTier(v as "pack" | "platform")} className="mb-4">
        <TabsList aria-label="Pack tier">
          <TabsTab value="pack">Packs ({counts.pack})</TabsTab>
          <TabsTab value="platform">Platform ({counts.platform})</TabsTab>
        </TabsList>
      </Tabs>

      <DataTable
        ariaLabel={tier === "pack" ? "Software packs" : "Platform components"}
        columns={columns}
        data={rows}
        getRowId={(r) => r.name}
        getRowLabel={(r) => r.name}
        filterColumnId="search"
        filterLabel="Search packs"
        filterPlaceholder="Search packs, charts or services…"
        initialPageSize={25}
        showPagination={rows.length > 25}
        loading={packs.isPending}
        error={packs.error ? packs.error.message : undefined}
        onRetry={() => void packs.refetch()}
        emptyTitle={tier === "pack" ? "No packs installed" : "No platform components"}
        emptyDescription="Packs appear once ArgoCD Applications labeled part-of=nebari-packs exist, or NebariApps carry Helm chart labels."
        filteredEmptyTitle="No packs match"
        filteredEmptyDescription="Try a different search."
      />
      <p className="mt-3 text-xs text-muted-foreground">
        {pluralize(rows.length, "entry", "entries")} · service health from the webapi probes, sync
        and health from ArgoCD.
      </p>
    </section>
  );
}
