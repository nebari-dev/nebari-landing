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

// Deterministic synthetic probe history: 288 samples at 5-minute spacing
// (24h). MLflow never probes; Grafana has one outage a few hours ago.
function mockHealthHistory(id: string) {
  if (id === "svc-mlflow") return [];
  const now = Date.now();
  const out: { at: string; status: string }[] = [];
  for (let i = 287; i >= 0; i--) {
    const at = new Date(now - i * 5 * 60 * 1000).toISOString();
    let status = "healthy";
    if (id === "svc-grafana" && i >= 40 && i < 46) status = "unhealthy";
    if (id === "svc-status" && i % 97 === 0) status = "unknown";
    if (id === "svc-vscode" && i < 3) status = "unhealthy";
    out.push({ at, status });
  }
  return out;
}

function mockHealth(id: string) {
  const history = mockHealthHistory(id);
  // Services without a health check carry no `health` at all.
  if (history.length === 0) return undefined;
  const last = history[history.length - 1];
  let since = last.at;
  for (let i = history.length - 1; i >= 0 && history[i].status === last.status; i--) {
    since = history[i].at;
  }
  const healthy = history.filter((h) => h.status === "healthy").length;
  return {
    status: last.status,
    lastCheck: last.at,
    message: last.status === "unhealthy" ? "HTTP 503 from /healthz" : "",
    samples: history.length,
    uptimePercent: (healthy * 100) / history.length,
    streakStatus: last.status,
    streakSince: since,
    windowStart: history[0].at,
  };
}

const PACKS: {
  name: string;
  tier: "pack" | "platform";
  namespace: string;
  chart: string;
  version: string;
  appVersion?: string;
  repo: string;
  sync: string;
  health: string;
  services: string[];
  images: string[];
}[] = [
  {
    name: "data-science-pack",
    tier: "pack",
    namespace: "jupyterhub",
    chart: "nebari-data-science-pack",
    version: "0.1.0-alpha.11",
    appVersion: "1.0.0",
    repo: "https://github.com/nebari-dev/nebari-data-science-pack.git",
    sync: "Synced",
    health: "Healthy",
    services: ["svc-jupyter", "svc-vscode"],
    images: [
      "quay.io/jupyterhub/k8s-hub:4.3.2",
      "quay.io/nebari/nebari-data-science-pack-jupyterlab:sha-5dfee5e",
    ],
  },
  {
    name: "lgtm-pack",
    tier: "pack",
    namespace: "monitoring",
    chart: "nebari-lgtm-pack",
    version: "0.2.0",
    appVersion: "1.0.0",
    repo: "https://nebari-dev.github.io/helm-repository",
    sync: "OutOfSync",
    health: "Healthy",
    services: ["svc-grafana"],
    images: ["grafana/grafana:11.4.0", "grafana/mimir:2.15.0"],
  },
  {
    name: "mlflow-pack",
    tier: "pack",
    namespace: "mlflow",
    chart: "nebari-mlflow-pack",
    version: "0.3.1",
    repo: "https://nebari-dev.github.io/helm-repository",
    sync: "Synced",
    health: "Degraded",
    services: ["svc-mlflow"],
    images: ["ghcr.io/mlflow/mlflow:v2.20.0"],
  },
  {
    name: "superset-pack",
    tier: "pack",
    namespace: "superset",
    chart: "nebari-superset-pack",
    version: "0.4.0",
    repo: "https://nebari-dev.github.io/helm-repository",
    sync: "Synced",
    health: "Healthy",
    services: ["svc-superset"],
    images: ["apache/superset:4.1.1"],
  },
  {
    name: "nebari-landingpage",
    tier: "platform",
    namespace: "nebari-system",
    chart: "nebari-landing",
    version: "0.1.5",
    appVersion: "0.1.5",
    repo: "https://github.com/nebari-dev/nebari-landing",
    sync: "Synced",
    health: "Healthy",
    services: ["svc-docs", "svc-status"],
    images: ["quay.io/nebari/nebari-landing:0.1.5", "quay.io/nebari/nebari-webapi:0.1.5"],
  },
  {
    name: "keycloak",
    tier: "platform",
    namespace: "keycloak",
    chart: "keycloakx",
    version: "7.1.6",
    repo: "https://codecentric.github.io/helm-charts",
    sync: "Synced",
    health: "Healthy",
    services: ["svc-keycloak"],
    images: ["quay.io/keycloak/keycloak:26.1.0"],
  },
];

function mockPacks() {
  const hours = (n: number) => new Date(Date.now() - n * 60 * 60 * 1000).toISOString();
  return PACKS.map((p, i) => ({
    name: p.name,
    tier: p.tier,
    namespace: p.namespace,
    chartName: p.chart,
    chartVersion: p.version,
    appVersion: p.appVersion,
    argo: {
      name: p.name,
      tier: p.tier,
      namespace: p.namespace,
      repoURL: p.repo,
      chart: p.chart,
      targetRevision: p.version,
      syncStatus: p.sync,
      healthStatus: p.health,
      revision: `${(i + 1).toString(16).padStart(2, "0")}ab34cd56ef7890123456789012345678901234`,
      lastSyncPhase: "Succeeded",
      lastSyncAt: hours(3 + i * 7),
      reconciledAt: hours(0.2),
      autoSync: true,
      images: p.images,
      resourceCount: 8 + i * 5,
    },
    services: p.services
      .map((id) => store.admin.services.find((s) => s.id === id))
      .filter((s): s is NonNullable<typeof s> => Boolean(s))
      .map((s) => ({
        id: s.id,
        name: s.name,
        displayName: s.displayName,
        namespace: s.namespace,
        url: s.url,
        visibility: s.visibility,
        requiredGroups: s.requiredGroups,
        healthStatus: mockHealth(s.id)?.status ?? "unknown",
      })),
  }));
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

  // --- overview ----------------------------------------------------------
  http.get(`${BASE}/overview`, () => {
    const users = store.admin.users;
    const weekAgo = Date.now() - 7 * 24 * 60 * 60 * 1000;
    const healthByName = new Map(store.services.map((s) => [s.name, s.status.toLowerCase()]));
    const services = store.admin.services;
    const health = services.map((s) => healthByName.get(s.displayName) ?? "unknown");
    return json(200, {
      generatedAt: new Date().toISOString(),
      identityAvailable: true,
      users: {
        total: users.length,
        enabled: users.filter((u) => u.enabled).length,
        disabled: users.filter((u) => !u.enabled).length,
        withoutGroups: users.filter((u) => u.groups.length === 0).length,
        createdLast7Days: users.filter((u) => Date.parse(u.createdAt) > weekAgo).length,
      },
      groups: store.admin.groups.length,
      roles: store.admin.roles.filter((r) => !r.builtIn).length,
      services: {
        total: services.length,
        healthy: health.filter((h) => h === "healthy").length,
        unhealthy: health.filter((h) => h === "unhealthy").length,
        unknown: health.filter((h) => h !== "healthy" && h !== "unhealthy").length,
        public: services.filter((s) => s.visibility === "public").length,
        gated: services.filter((s) => s.visibility !== "public" && s.requiredGroups.length > 0)
          .length,
      },
      accessRequestsAvailable: true,
      accessRequests: {
        pending: store.accessRequests.filter((r) => r.status === "pending").length,
        approved: store.accessRequests.filter((r) => r.status === "approved").length,
        denied: store.accessRequests.filter((r) => r.status === "denied").length,
      },
      sessions: {
        users: 5,
        sessions: 11,
        clients: [
          { clientId: "nebari-frontend-spa", active: 5 },
          { clientId: "jupyterhub", active: 3 },
          { clientId: "grafana", active: 2 },
          { clientId: "superset", active: 1 },
        ],
      },
    });
  }),

  // --- packs (read-only) ------------------------------------------------
  http.get(`${BASE}/packs`, () => json(200, { argocdAvailable: true, packs: mockPacks() })),
  http.get(`${BASE}/packs/:name`, ({ params }) => {
    const pack = mockPacks().find((p) => p.name === params.name);
    return pack ? json(200, pack) : problem(404, "pack not found");
  }),

  // --- services (read-only) ---------------------------------------------
  http.get(`${BASE}/services`, () =>
    json(
      200,
      store.admin.services.map((s) => ({ ...s, health: mockHealth(s.id) })),
    ),
  ),

  http.get(`${BASE}/services/:id`, ({ params }) => {
    const service = store.admin.services.find((s) => s.id === params.id);
    return service
      ? json(200, { ...service, health: mockHealth(service.id) })
      : problem(404, "service not found");
  }),

  http.get(`${BASE}/services/:id/health`, ({ params }) => {
    const service = store.admin.services.find((s) => s.id === params.id);
    if (!service) return problem(404, "service not found");
    const history = mockHealthHistory(service.id);
    return json(200, {
      ...(mockHealth(service.id) ?? { status: "unknown", samples: 0, uptimePercent: null }),
      history,
    });
  }),
];
