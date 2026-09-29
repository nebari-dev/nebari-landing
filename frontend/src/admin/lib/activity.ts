import type { AdminGroup, AdminUser, Pack } from "../api/types";

export type ActivityKind = "account" | "sync";

export type ActivityEvent = {
  key: string;
  kind: ActivityKind;
  at: string;
  event: string;
  /** Route to the subject's page. */
  to: string;
  subject: string;
  /** Group names for account events, chart/revision text for syncs. */
  groups: string[];
  details: string;
};

/**
 * Derives an activity feed from live objects. There is no event store yet,
 * so only what is stamped on users (createdTimestamp) and ArgoCD
 * Applications (last operation) can appear here.
 */
export function buildActivity(
  users: AdminUser[],
  groups: AdminGroup[],
  packs: Pack[],
): ActivityEvent[] {
  const groupName = new Map(groups.map((g) => [g.id, g.name]));
  const out: ActivityEvent[] = [];
  for (const u of users) {
    if (!u.createdAt) continue;
    out.push({
      key: `user-${u.id}`,
      kind: "account",
      at: u.createdAt,
      event: "Account created",
      to: `/admin/users/${encodeURIComponent(u.id)}`,
      subject: `${u.firstName} ${u.lastName}`.trim() || u.username,
      groups: u.groups.map((gid) => groupName.get(gid) ?? gid),
      details: u.groups.length === 0 ? "no groups" : "",
    });
  }
  for (const p of packs) {
    if (!p.argo?.lastSyncAt) continue;
    out.push({
      key: `sync-${p.name}`,
      kind: "sync",
      at: p.argo.lastSyncAt,
      event: `Pack sync ${p.argo.lastSyncPhase?.toLowerCase() || "finished"}`,
      to: `/admin/packs/${encodeURIComponent(p.name)}`,
      subject: p.name,
      groups: [],
      details: [p.chartName, p.chartVersion, p.argo.revision ? p.argo.revision.slice(0, 7) : ""]
        .filter(Boolean)
        .join(" · "),
    });
  }
  return out.sort((a, b) => b.at.localeCompare(a.at));
}
