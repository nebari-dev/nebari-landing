// Seed data for the admin (identity) mocks. Deterministic so unit tests and
// screenshots are stable. Group ids are referenced by users; services
// reference groups by *name*, exactly as NebariApp.requiredGroups does.

import type { AdminGroup, AdminRole, AdminService, AdminUser } from "@/admin/api/types";

const T0 = Date.UTC(2026, 0, 6, 9, 0, 0); // 2026-01-06T09:00:00Z
const DAY = 24 * 60 * 60 * 1000;

function iso(daysFromT0: number, hour = 9): string {
  return new Date(T0 + daysFromT0 * DAY + (hour - 9) * 60 * 60 * 1000).toISOString();
}

export const seedRoles: AdminRole[] = [
  { name: "admin", description: "Full platform administration.", composite: true, builtIn: false },
  {
    name: "developer",
    description: "Deploy and debug workloads.",
    composite: false,
    builtIn: false,
  },
  {
    name: "viewer",
    description: "Read-only access to dashboards.",
    composite: false,
    builtIn: false,
  },
  {
    name: "dashboard-editor",
    description: "Create and edit Superset dashboards.",
    composite: false,
    builtIn: false,
  },
  {
    name: "jupyterhub-user",
    description: "May spawn a JupyterHub server.",
    composite: false,
    builtIn: false,
  },
  {
    name: "allow-group-directory-creation-role",
    description: "Data-science pack: group gets a shared directory.",
    composite: false,
    builtIn: false,
  },
  { name: "offline_access", description: "", composite: false, builtIn: true },
  { name: "uma_authorization", description: "", composite: false, builtIn: true },
  { name: "default-roles-nebari", description: "", composite: true, builtIn: true },
];

export const seedGroups: AdminGroup[] = [
  {
    id: "grp-admin",
    name: "admin",
    path: "/admin",
    description: "Platform administrators.",
    roles: ["admin"],
    createdAt: iso(-400),
  },
  {
    id: "grp-users",
    name: "users",
    path: "/users",
    description: "Every human account. Grants the baseline services.",
    roles: ["jupyterhub-user"],
    createdAt: iso(-400),
  },
  {
    id: "grp-developers",
    name: "developers",
    path: "/developers",
    description: "Engineers shipping code on the platform.",
    roles: ["developer"],
    createdAt: iso(-320),
  },
  {
    id: "grp-data-science",
    name: "data-science",
    path: "/data-science",
    description: "Data-science team; gets a shared directory.",
    roles: ["allow-group-directory-creation-role"],
    createdAt: iso(-300),
  },
  {
    id: "grp-analysts",
    name: "analysts",
    path: "/analysts",
    description: "Business analysts working in Superset.",
    roles: ["dashboard-editor", "viewer"],
    createdAt: iso(-210),
  },
  {
    id: "grp-ml-engineers",
    name: "ml-engineers",
    path: "/ml-engineers",
    description: "Model training and serving.",
    roles: ["developer"],
    createdAt: iso(-150),
  },
  {
    id: "grp-finance",
    name: "finance",
    path: "/finance",
    description: "Finance viewers.",
    roles: ["viewer"],
    createdAt: iso(-90),
  },
  {
    id: "grp-contractors",
    name: "contractors",
    path: "/contractors",
    description: "Time-boxed external accounts.",
    roles: [],
    createdAt: iso(-40),
  },
];

export const seedServices: AdminService[] = [
  {
    id: "svc-jupyter",
    name: "jupyterhub",
    displayName: "JupyterHub",
    namespace: "jupyterhub",
    category: "Notebooks",
    url: "https://jupyter.example.com",
    visibility: "private",
    requiredGroups: ["users"],
    docsUrl: "https://docs.nebari.dev/packs/jupyterhub",
    settingsUrl: "https://jupyter.example.com/hub/admin",
  },
  {
    id: "svc-vscode",
    name: "code-server",
    displayName: "VS Code Server",
    namespace: "code-server",
    category: "IDE",
    url: "https://code.example.com",
    visibility: "private",
    requiredGroups: ["developers", "ml-engineers"],
  },
  {
    id: "svc-grafana",
    name: "grafana",
    displayName: "Grafana",
    namespace: "monitoring",
    category: "Monitoring",
    url: "https://grafana.example.com",
    visibility: "private",
    requiredGroups: ["admin", "developers"],
    docsUrl: "https://docs.nebari.dev/packs/observability",
  },
  {
    id: "svc-mlflow",
    name: "mlflow",
    displayName: "MLflow",
    namespace: "mlflow",
    category: "ML",
    url: "https://mlflow.example.com",
    visibility: "private",
    requiredGroups: ["ml-engineers", "data-science"],
    docsUrl: "https://docs.nebari.dev/packs/mlflow",
  },
  {
    id: "svc-superset",
    name: "superset",
    displayName: "Superset",
    namespace: "superset",
    category: "Analytics",
    url: "https://superset.example.com",
    visibility: "private",
    requiredGroups: ["analysts", "finance"],
    docsUrl: "https://docs.nebari.dev/packs/superset",
    settingsUrl: "https://superset.example.com/roles/list/",
  },
  {
    id: "svc-keycloak",
    name: "keycloak",
    displayName: "Keycloak",
    namespace: "keycloak",
    category: "Platform",
    url: "https://auth.example.com/admin/nebari/console/",
    visibility: "private",
    requiredGroups: ["admin"],
  },
  {
    id: "svc-docs",
    name: "docs",
    displayName: "Platform Docs",
    namespace: "docs",
    category: "Documentation",
    url: "https://docs.example.com",
    visibility: "public",
    requiredGroups: [],
  },
  {
    id: "svc-status",
    name: "status",
    displayName: "Status Page",
    namespace: "status",
    category: "Platform",
    url: "https://status.example.com",
    visibility: "private",
    requiredGroups: [],
  },
];

const FIRST = [
  "Alice",
  "Bruno",
  "Chen",
  "Dana",
  "Elias",
  "Farah",
  "Gabriel",
  "Hana",
  "Ivan",
  "Jun",
  "Kai",
  "Lena",
  "Mateo",
  "Nia",
  "Omar",
  "Priya",
  "Quinn",
  "Rosa",
  "Sami",
  "Tomás",
];
const LAST = [
  "Alvarez",
  "Brennan",
  "Castillo",
  "Dubois",
  "Eriksen",
  "Fischer",
  "Gomez",
  "Haddad",
  "Ito",
  "Jensen",
  "Kowalski",
  "Lindqvist",
];

/** Named accounts referenced by tests and screenshots; the rest are generated. */
const NAMED: Partial<AdminUser>[] = [
  {
    id: "usr-alice",
    username: "alice",
    firstName: "Alice",
    lastName: "Alvarez",
    email: "alice@example.com",
    groups: ["grp-users", "grp-admin", "grp-developers"],
    roles: [],
    lastSignInAt: iso(260, 8),
  },
  {
    id: "usr-bruno",
    username: "bruno",
    firstName: "Bruno",
    lastName: "Brennan",
    email: "bruno@example.com",
    groups: ["grp-users", "grp-data-science"],
    roles: ["viewer"],
    lastSignInAt: iso(259, 17),
  },
  {
    id: "usr-chen",
    username: "chen",
    firstName: "Chen",
    lastName: "Castillo",
    email: "chen@example.com",
    groups: ["grp-users", "grp-ml-engineers"],
    roles: [],
    lastSignInAt: iso(255, 11),
  },
  {
    id: "usr-dana",
    username: "dana",
    firstName: "Dana",
    lastName: "Dubois",
    email: "dana@example.com",
    groups: ["grp-users", "grp-analysts"],
    roles: [],
    lastSignInAt: null,
  },
  {
    id: "usr-elias",
    username: "elias",
    firstName: "Elias",
    lastName: "Eriksen",
    email: "elias.eriksen@contractor.example.com",
    groups: ["grp-contractors"],
    roles: [],
    enabled: false,
    lastSignInAt: iso(120, 14),
  },
];

const GROUP_CYCLE: string[][] = [
  ["grp-users"],
  ["grp-users", "grp-developers"],
  ["grp-users", "grp-analysts"],
  ["grp-users", "grp-data-science"],
  ["grp-users", "grp-ml-engineers", "grp-developers"],
  ["grp-users", "grp-finance"],
  ["grp-contractors"],
  ["grp-users", "grp-analysts", "grp-finance"],
];

function generateUsers(count: number): AdminUser[] {
  const users: AdminUser[] = [];
  for (let i = 0; i < count; i++) {
    const named = NAMED[i];
    const firstName = named?.firstName ?? FIRST[i % FIRST.length];
    const lastName = named?.lastName ?? LAST[Math.floor(i / FIRST.length + i) % LAST.length];
    const username = named?.username ?? `${firstName}.${lastName}${i >= 20 ? i : ""}`.toLowerCase();
    users.push({
      id: named?.id ?? `usr-${String(i + 1).padStart(3, "0")}`,
      username,
      email: named?.email ?? `${username}@example.com`,
      firstName,
      lastName,
      enabled: named?.enabled ?? i % 13 !== 12,
      createdAt: iso(-360 + i * 5),
      lastSignInAt:
        named?.lastSignInAt !== undefined
          ? named.lastSignInAt
          : i % 7 === 6
            ? null
            : iso(200 + (i % 50), 8 + (i % 9)),
      groups: named?.groups ?? GROUP_CYCLE[i % GROUP_CYCLE.length],
      roles: named?.roles ?? (i % 11 === 10 ? ["viewer"] : []),
    });
  }
  return users;
}

export const seedUsers: AdminUser[] = generateUsers(60);
