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
