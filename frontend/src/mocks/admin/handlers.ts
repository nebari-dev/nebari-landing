// Stateful MSW handlers for the proposed /api/v1/admin/{users,groups,roles,
// services} endpoints. Not in the OpenAPI spec yet, so they live here rather
// than in the generated layer. See docs/design/admin-user-management.md.

import { HttpResponse, http, type JsonBodyType } from "msw";
import type { AdminGroup, AdminRole, AdminUser, BulkUsersRequest } from "@/admin/api/types";
import { store } from "../store";

const BASE = "/api/v1/admin";

function json(status: number, body: JsonBodyType) {
  return HttpResponse.json(body, { status });
}

function problem(status: number, message: string) {
  return json(status, { error: message });
}

function slug(name: string): string {
  return name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

type PathParam = string | readonly string[] | undefined;

function findUser(id: PathParam): AdminUser | undefined {
  return store.admin.users.find((u) => u.id === id);
}

function findGroup(id: PathParam): AdminGroup | undefined {
  return store.admin.groups.find((g) => g.id === id);
}

function findRole(name: PathParam): AdminRole | undefined {
  return store.admin.roles.find((r) => r.name === name);
}

function addUnique(list: string[], value: string) {
  if (!list.includes(value)) list.push(value);
}

function remove(list: string[], value: string) {
  const i = list.indexOf(value);
  if (i >= 0) list.splice(i, 1);
}

function servicesReferencingGroup(name: string) {
  return store.admin.services.filter((s) => s.requiredGroups.includes(name));
}

export const adminHandlers = [
  // --- users -------------------------------------------------------------
  http.get(`${BASE}/users`, ({ request }) => {
    const url = new URL(request.url);
    const q = url.searchParams.get("q")?.toLowerCase() ?? "";
    const group = url.searchParams.get("group");
    const role = url.searchParams.get("role");
    const enabled = url.searchParams.get("enabled");

    let users = store.admin.users;
    if (q) {
      users = users.filter((u) =>
        [u.username, u.email, u.firstName, u.lastName].some((f) => f.toLowerCase().includes(q)),
      );
    }
    if (group) users = users.filter((u) => u.groups.includes(group));
    if (role) users = users.filter((u) => u.roles.includes(role));
    if (enabled === "true" || enabled === "false") {
      users = users.filter((u) => u.enabled === (enabled === "true"));
    }

    const total = users.length;
    const page = Number(url.searchParams.get("page") ?? "0");
    const pageSize = Number(url.searchParams.get("pageSize") ?? "0");
    if (pageSize > 0) users = users.slice(page * pageSize, (page + 1) * pageSize);

    return json(200, { users, total });
  }),

  http.get(`${BASE}/users/:id`, ({ params }) => {
    const user = findUser(params.id);
    return user ? json(200, user) : problem(404, "user not found");
  }),

  http.patch(`${BASE}/users/:id`, async ({ params, request }) => {
    const user = findUser(params.id);
    if (!user) return problem(404, "user not found");
    const body = (await request.json().catch(() => ({}))) as Partial<Pick<AdminUser, "enabled">>;
    if (typeof body.enabled === "boolean") user.enabled = body.enabled;
    return json(200, user);
  }),

  http.delete(`${BASE}/users/:id`, ({ params }) => {
    const user = findUser(params.id);
    if (!user) return problem(404, "user not found");
    if (user.username === "dev") return problem(403, "you cannot delete your own account");
    store.admin.users = store.admin.users.filter((u) => u.id !== user.id);
    return new HttpResponse(null, { status: 204 });
  }),

  http.put(`${BASE}/users/:id/groups/:groupId`, ({ params }) => {
    const user = findUser(params.id);
    const group = findGroup(params.groupId);
    if (!user || !group) return problem(404, "user or group not found");
    addUnique(user.groups, group.id);
    return json(200, user);
  }),

  http.delete(`${BASE}/users/:id/groups/:groupId`, ({ params }) => {
    const user = findUser(params.id);
    if (!user) return problem(404, "user not found");
    remove(user.groups, String(params.groupId));
    return json(200, user);
  }),

  http.put(`${BASE}/users/:id/roles/:role`, ({ params }) => {
    const user = findUser(params.id);
    const role = findRole(params.role);
    if (!user || !role) return problem(404, "user or role not found");
    addUnique(user.roles, role.name);
    return json(200, user);
  }),

  http.delete(`${BASE}/users/:id/roles/:role`, ({ params }) => {
    const user = findUser(params.id);
    if (!user) return problem(404, "user not found");
    remove(user.roles, String(params.role));
    return json(200, user);
  }),

  http.post(`${BASE}/users/bulk`, async ({ request }) => {
    const body = (await request.json().catch(() => null)) as BulkUsersRequest | null;
    if (!body || !Array.isArray(body.userIds)) return problem(400, "invalid request");

    if ((body.action === "addToGroup" || body.action === "removeFromGroup") && !body.groupId) {
      return problem(400, "groupId required");
    }
    if ((body.action === "assignRole" || body.action === "unassignRole") && !body.role) {
      return problem(400, "role required");
    }
    if (body.groupId && !findGroup(body.groupId)) return problem(404, "group not found");
    if (body.role && !findRole(body.role)) return problem(404, "role not found");

    let updated = 0;
    for (const id of body.userIds) {
      const user = findUser(id);
      if (!user) continue;
      updated++;
      switch (body.action) {
        case "addToGroup":
          addUnique(user.groups, body.groupId as string);
          break;
        case "removeFromGroup":
          remove(user.groups, body.groupId as string);
          break;
        case "assignRole":
          addUnique(user.roles, body.role as string);
          break;
        case "unassignRole":
          remove(user.roles, body.role as string);
          break;
        case "enable":
          user.enabled = true;
          break;
        case "disable":
          user.enabled = false;
          break;
      }
    }
    return json(200, { updated });
  }),

  // --- groups ------------------------------------------------------------
  http.get(`${BASE}/groups`, () => json(200, store.admin.groups)),

  http.post(`${BASE}/groups`, async ({ request }) => {
    const body = (await request.json().catch(() => ({}))) as Partial<AdminGroup>;
    const name = slug(body.name ?? "");
    if (!name) return problem(400, "name required");
    if (store.admin.groups.some((g) => g.name === name)) {
      return problem(409, `group "${name}" already exists`);
    }
    const group: AdminGroup = {
      id: `grp-${name}`,
      name,
      path: `/${name}`,
      description: body.description ?? "",
      roles: [],
      createdAt: new Date().toISOString(),
    };
    store.admin.groups.push(group);
    return json(201, group);
  }),

  http.get(`${BASE}/groups/:id`, ({ params }) => {
    const group = findGroup(params.id);
    return group ? json(200, group) : problem(404, "group not found");
  }),

  http.patch(`${BASE}/groups/:id`, async ({ params, request }) => {
    const group = findGroup(params.id);
    if (!group) return problem(404, "group not found");
    const body = (await request.json().catch(() => ({}))) as Partial<AdminGroup>;
    if (typeof body.description === "string") group.description = body.description;
    return json(200, group);
  }),

  http.delete(`${BASE}/groups/:id`, ({ params }) => {
    const group = findGroup(params.id);
    if (!group) return problem(404, "group not found");
    const refs = servicesReferencingGroup(group.name);
    if (refs.length > 0) {
      return json(409, {
        error: "group is referenced by NebariApp.requiredGroups",
        services: refs.map((s) => s.id),
      });
    }
    store.admin.groups = store.admin.groups.filter((g) => g.id !== group.id);
    for (const user of store.admin.users) remove(user.groups, group.id);
    return new HttpResponse(null, { status: 204 });
  }),

  http.put(`${BASE}/groups/:id/members/:userId`, ({ params }) => {
    const group = findGroup(params.id);
    const user = findUser(params.userId);
    if (!group || !user) return problem(404, "group or user not found");
    addUnique(user.groups, group.id);
    return json(200, group);
  }),

  http.delete(`${BASE}/groups/:id/members/:userId`, ({ params }) => {
    const group = findGroup(params.id);
    const user = findUser(params.userId);
    if (!group || !user) return problem(404, "group or user not found");
    remove(user.groups, group.id);
    return json(200, group);
  }),

  http.put(`${BASE}/groups/:id/roles/:role`, ({ params }) => {
    const group = findGroup(params.id);
    const role = findRole(params.role);
    if (!group || !role) return problem(404, "group or role not found");
    addUnique(group.roles, role.name);
    return json(200, group);
  }),

  http.delete(`${BASE}/groups/:id/roles/:role`, ({ params }) => {
    const group = findGroup(params.id);
    if (!group) return problem(404, "group not found");
    remove(group.roles, String(params.role));
    return json(200, group);
  }),

  // --- roles -------------------------------------------------------------
  http.get(`${BASE}/roles`, () => json(200, store.admin.roles)),

  http.post(`${BASE}/roles`, async ({ request }) => {
    const body = (await request.json().catch(() => ({}))) as Partial<AdminRole>;
    const name = slug(body.name ?? "");
    if (!name) return problem(400, "name required");
    if (findRole(name)) return problem(409, `role "${name}" already exists`);
    const role: AdminRole = {
      name,
      description: body.description ?? "",
      composite: false,
      builtIn: false,
    };
    store.admin.roles.push(role);
    return json(201, role);
  }),

  http.get(`${BASE}/roles/:name`, ({ params }) => {
    const role = findRole(params.name);
    return role ? json(200, role) : problem(404, "role not found");
  }),

  http.patch(`${BASE}/roles/:name`, async ({ params, request }) => {
    const role = findRole(params.name);
    if (!role) return problem(404, "role not found");
    if (role.builtIn) return problem(403, "built-in roles cannot be edited");
    const body = (await request.json().catch(() => ({}))) as Partial<AdminRole>;
    if (typeof body.description === "string") role.description = body.description;
    return json(200, role);
  }),

  http.delete(`${BASE}/roles/:name`, ({ params }) => {
    const role = findRole(params.name);
    if (!role) return problem(404, "role not found");
    if (role.builtIn) return problem(403, "built-in roles cannot be deleted");
    store.admin.roles = store.admin.roles.filter((r) => r.name !== role.name);
    for (const user of store.admin.users) remove(user.roles, role.name);
    for (const group of store.admin.groups) remove(group.roles, role.name);
    return new HttpResponse(null, { status: 204 });
  }),

  // --- services (read-only) ---------------------------------------------
  http.get(`${BASE}/services`, () => json(200, store.admin.services)),

  http.get(`${BASE}/services/:id`, ({ params }) => {
    const service = store.admin.services.find((s) => s.id === params.id);
    return service ? json(200, service) : problem(404, "service not found");
  }),
];
