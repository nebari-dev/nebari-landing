import type { AdminGroup, AdminService, AdminUser } from "@/admin/api/types";
import {
  canAccess,
  effectiveRoles,
  effectiveUserCount,
  principalsForService,
  servicesForUser,
  servicesUnlockedByGroup,
} from "@/admin/lib/access";
import { describe, expect, it } from "vitest";

const groups: AdminGroup[] = [
  { id: "g-a", name: "analysts", path: "/analysts", description: "", roles: ["viewer"], createdAt: "" },
  { id: "g-d", name: "devs", path: "/devs", description: "", roles: [], createdAt: "" },
];

const services: AdminService[] = [
  { id: "s-pub", name: "docs", displayName: "Docs", namespace: "n", category: "c", url: "", visibility: "public", requiredGroups: [] },
  { id: "s-any", name: "status", displayName: "Status", namespace: "n", category: "c", url: "", visibility: "private", requiredGroups: [] },
  { id: "s-gated", name: "superset", displayName: "Superset", namespace: "n", category: "c", url: "", visibility: "private", requiredGroups: ["analysts", "ghost"] },
];

function user(overrides: Partial<AdminUser>): AdminUser {
  return {
    id: "u",
    username: "u",
    email: "u@example.com",
    firstName: "U",
    lastName: "Ser",
    enabled: true,
    createdAt: "",
    lastSignInAt: null,
    groups: [],
    roles: [],
    ...overrides,
  };
}

describe("canAccess mirrors canAccessPolicy", () => {
  it("public is open to everyone, even anonymous", () => {
    expect(canAccess(services[0], [], false)).toBe(true);
  });

  it("private without groups needs authentication only", () => {
    expect(canAccess(services[1], [], false)).toBe(false);
    expect(canAccess(services[1], [], true)).toBe(true);
  });

  it("private with groups needs an intersecting group", () => {
    expect(canAccess(services[2], ["devs"])).toBe(false);
    expect(canAccess(services[2], ["devs", "analysts"])).toBe(true);
  });
});

describe("servicesForUser", () => {
  it("explains why each service is reachable", () => {
    const access = servicesForUser(user({ groups: ["g-a"] }), groups, services);
    expect(access.map((a) => [a.service.id, a.reason.kind])).toEqual([
      ["s-pub", "public"],
      ["s-any", "any-authenticated"],
      ["s-gated", "groups"],
    ]);
    const gated = access.find((a) => a.service.id === "s-gated");
    expect(gated?.reason).toEqual({ kind: "groups", groups: ["analysts"] });
  });

  it("omits gated services the user's groups don't unlock", () => {
    const access = servicesForUser(user({ groups: ["g-d"] }), groups, services);
    expect(access.map((a) => a.service.id)).toEqual(["s-pub", "s-any"]);
  });
});

describe("servicesUnlockedByGroup / principalsForService", () => {
  const users = [
    user({ id: "u1", groups: ["g-a"] }),
    user({ id: "u2", groups: ["g-a"], enabled: false }),
    user({ id: "u3", groups: ["g-d"] }),
  ];

  it("lists services that name the group", () => {
    expect(servicesUnlockedByGroup(groups[0], services).map((s) => s.id)).toEqual(["s-gated"]);
    expect(servicesUnlockedByGroup(groups[1], services)).toEqual([]);
  });

  it("expands the gate to members and flags groups missing from Keycloak", () => {
    const principals = principalsForService(services[2], groups, users);
    expect(principals.kind).toBe("groups");
    if (principals.kind !== "groups") throw new Error("unreachable");
    expect(principals.entries.map((e) => [e.name, e.group?.id ?? null, e.members.length])).toEqual(
      [
        ["analysts", "g-a", 2],
        ["ghost", null, 0],
      ],
    );
  });

  it("counts distinct enabled users, or everyone for public services", () => {
    expect(effectiveUserCount(services[0], groups, users)).toBe("everyone");
    expect(effectiveUserCount(services[1], groups, users)).toBe(2);
    expect(effectiveUserCount(services[2], groups, users)).toBe(1);
  });
});

describe("effectiveRoles", () => {
  it("separates direct from group-inherited roles without duplicating", () => {
    const info = effectiveRoles(user({ groups: ["g-a"], roles: ["admin"] }), groups);
    expect(info.direct).toEqual(["admin"]);
    expect(info.inherited).toEqual([{ role: "viewer", viaGroup: groups[0] }]);

    const dup = effectiveRoles(user({ groups: ["g-a"], roles: ["viewer"] }), groups);
    expect(dup.inherited).toEqual([]);
  });
});
