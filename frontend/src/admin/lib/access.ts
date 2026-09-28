import type { AdminGroup, AdminService, AdminUser } from "../api/types";

/**
 * Pure access rule, mirroring `canAccessPolicy` in internal/api/handlers.go:
 *
 *   public                       → everyone
 *   private + no requiredGroups  → any authenticated user
 *   private + requiredGroups     → authenticated users in at least one group
 */
export function canAccess(
  service: Pick<AdminService, "visibility" | "requiredGroups">,
  userGroupNames: string[],
  authenticated = true,
): boolean {
  if (service.visibility === "public") return true;
  if (!authenticated) return false;
  if (service.requiredGroups.length === 0) return true;
  return service.requiredGroups.some((g) => userGroupNames.includes(g));
}

export type AccessReason =
  | { kind: "public" }
  | { kind: "any-authenticated" }
  | { kind: "groups"; groups: string[] };

export type ServiceAccess = {
  service: AdminService;
  reason: AccessReason;
};

/** Resolve a user's group ids to bare group names. */
export function groupNamesForUser(user: Pick<AdminUser, "groups">, groups: AdminGroup[]): string[] {
  const byId = new Map(groups.map((g) => [g.id, g.name]));
  return user.groups.map((id) => byId.get(id)).filter((n): n is string => n !== undefined);
}

/** Services a user can reach, and why. */
export function servicesForUser(
  user: Pick<AdminUser, "groups">,
  groups: AdminGroup[],
  services: AdminService[],
): ServiceAccess[] {
  const names = groupNamesForUser(user, groups);
  return servicesForGroupNames(names, services);
}

/** Services reachable by a principal holding exactly these groups. */
export function servicesForGroupNames(names: string[], services: AdminService[]): ServiceAccess[] {
  const out: ServiceAccess[] = [];
  for (const service of services) {
    if (service.visibility === "public") {
      out.push({ service, reason: { kind: "public" } });
      continue;
    }
    if (service.requiredGroups.length === 0) {
      out.push({ service, reason: { kind: "any-authenticated" } });
      continue;
    }
    const via = service.requiredGroups.filter((g) => names.includes(g));
    if (via.length > 0) out.push({ service, reason: { kind: "groups", groups: via } });
  }
  return out;
}

/** Services whose gate names this group explicitly. */
export function servicesUnlockedByGroup(
  group: Pick<AdminGroup, "name">,
  services: AdminService[],
): AdminService[] {
  return services.filter((s) => s.requiredGroups.includes(group.name));
}

export type ServicePrincipals =
  | { kind: "everyone" }
  | { kind: "any-authenticated"; userCount: number }
  | { kind: "groups"; entries: { group: AdminGroup | null; name: string; members: AdminUser[] }[] };

/** Who can reach a service: the groups in its gate, expanded to members. */
export function principalsForService(
  service: AdminService,
  groups: AdminGroup[],
  users: AdminUser[],
): ServicePrincipals {
  if (service.visibility === "public") return { kind: "everyone" };
  if (service.requiredGroups.length === 0) {
    return { kind: "any-authenticated", userCount: users.filter((u) => u.enabled).length };
  }
  const entries = service.requiredGroups.map((name) => {
    const group = groups.find((g) => g.name === name) ?? null;
    const members = group ? users.filter((u) => u.groups.includes(group.id)) : [];
    return { group, name, members };
  });
  return { kind: "groups", entries };
}

/** Distinct enabled users who can reach a service. */
export function effectiveUserCount(
  service: AdminService,
  groups: AdminGroup[],
  users: AdminUser[],
): number | "everyone" {
  const principals = principalsForService(service, groups, users);
  if (principals.kind === "everyone") return "everyone";
  if (principals.kind === "any-authenticated") return principals.userCount;
  const ids = new Set<string>();
  for (const entry of principals.entries) {
    for (const u of entry.members) if (u.enabled) ids.add(u.id);
  }
  return ids.size;
}

/** Realm roles a user holds, split by where they come from. */
export function effectiveRoles(
  user: Pick<AdminUser, "roles" | "groups">,
  groups: AdminGroup[],
): { direct: string[]; inherited: { role: string; viaGroup: AdminGroup }[] } {
  const inherited: { role: string; viaGroup: AdminGroup }[] = [];
  for (const group of groups) {
    if (!user.groups.includes(group.id)) continue;
    for (const role of group.roles) {
      if (!user.roles.includes(role)) inherited.push({ role, viaGroup: group });
    }
  }
  return { direct: [...user.roles], inherited };
}
