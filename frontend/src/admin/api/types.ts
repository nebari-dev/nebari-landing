// Types for the proposed `/api/v1/admin/*` identity endpoints. These are the
// shapes the MSW layer serves today and the contract a future webapi
// implementation (backed by the existing gocloak admin client) would satisfy.
// See docs/design/admin-user-management.md.

export type AdminUser = {
  id: string;
  username: string;
  email: string;
  firstName: string;
  lastName: string;
  enabled: boolean;
  createdAt: string;
  /** ISO timestamp of the last successful sign-in, or null if never. */
  lastSignInAt: string | null;
  /** Group ids the user is a direct member of. */
  groups: string[];
  /** Realm role names mapped directly to the user (not via groups). */
  roles: string[];
};

export type AdminGroup = {
  id: string;
  /** Bare group name as it appears in the JWT `groups` claim. */
  name: string;
  /** Keycloak path, e.g. "/developers". */
  path: string;
  description: string;
  /** Realm role names mapped to the group; members inherit them. */
  roles: string[];
  createdAt: string;
};

export type AdminRole = {
  name: string;
  description: string;
  composite: boolean;
  /** Keycloak-managed roles (offline_access, uma_authorization, default-roles-*). */
  builtIn: boolean;
};

export type ServiceVisibility = "public" | "private";

/**
 * Read-only view of a `NebariApp` as the admin area sees it. Sourced from the
 * watcher cache; `requiredGroups` is the routing gate declared in the CR and
 * cannot be edited from the UI (GitOps owns it).
 */
export type AdminService = {
  id: string;
  name: string;
  displayName: string;
  namespace: string;
  category: string;
  url: string;
  visibility: ServiceVisibility;
  /** Bare group names. Empty with `visibility: private` means any signed-in user. */
  requiredGroups: string[];
  /** Optional pack-registered links (proposed CRD fields). */
  docsUrl?: string;
  settingsUrl?: string;
  /** Latest probe plus a summary of the retained history; absent without a health check. */
  health?: AdminServiceHealth;
};

export type HealthStatusValue = "healthy" | "unhealthy" | "unknown";

export type HealthSample = { at: string; status: HealthStatusValue | string };

export type AdminServiceHealth = {
  status: HealthStatusValue | string;
  lastCheck?: string;
  message?: string;
  samples: number;
  uptimePercent: number | null;
  streakStatus?: string;
  streakSince?: string;
  windowStart?: string;
};

export type AdminServiceHealthHistory = AdminServiceHealth & { history: HealthSample[] };

/** Mirrors `api.AdminOverview` from the webapi. */
export type AdminOverview = {
  generatedAt: string;
  identityAvailable: boolean;
  users: {
    total: number;
    enabled: number;
    disabled: number;
    withoutGroups: number;
    createdLast7Days: number;
  };
  groups: number;
  roles: number;
  services: {
    total: number;
    healthy: number;
    unhealthy: number;
    unknown: number;
    public: number;
    gated: number;
  };
  accessRequestsAvailable: boolean;
  accessRequests: { pending: number; approved: number; denied: number };
  /** Who is online: distinct users, raw sessions, per-client breakdown; null when unreadable. */
  sessions: {
    users: number;
    sessions: number;
    clients: { clientId: string; active: number }[];
  } | null;
};

/** Mirrors `packs.ArgoApp`. */
export type ArgoApp = {
  name: string;
  tier: "pack" | "platform";
  namespace: string;
  repoURL: string;
  chart?: string;
  path?: string;
  targetRevision: string;
  syncStatus: string;
  healthStatus: string;
  revision?: string;
  lastSyncPhase?: string;
  lastSyncAt: string;
  reconciledAt: string;
  autoSync: boolean;
  images: string[];
  resourceCount: number;
};

export type PackService = {
  id: string;
  name: string;
  displayName: string;
  namespace: string;
  url: string;
  visibility: ServiceVisibility;
  requiredGroups: string[];
  healthStatus: string;
};

/** Mirrors `packs.Pack`. */
export type Pack = {
  name: string;
  tier: "pack" | "platform";
  namespace: string;
  chartName?: string;
  chartVersion?: string;
  appVersion?: string;
  argo?: ArgoApp;
  /** Newest version published in the pack's Helm repository, when resolvable. */
  latestVersion?: string;
  versionStatus: "current" | "behind" | "ahead" | "unknown";
  services: PackService[];
};

export type AdminPacksResponse = {
  argocdAvailable: boolean;
  error?: string;
  packs: Pack[];
};

export type UsersListResponse = {
  users: AdminUser[];
  total: number;
};

export type BulkUserAction =
  | "addToGroup"
  | "removeFromGroup"
  | "assignRole"
  | "unassignRole"
  | "enable"
  | "disable";

export type BulkUsersRequest = {
  userIds: string[];
  action: BulkUserAction;
  groupId?: string;
  role?: string;
};

export type BulkUsersResponse = {
  updated: number;
};
