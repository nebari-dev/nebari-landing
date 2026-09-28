// Module-scoped, in-memory state used by the stateful MSW handlers. Tests
// reset between cases via resetStore(); the browser worker resets once on
// load. There is no persistence — refreshing the page rehydrates from the
// seed fixtures.

import type { AdminGroup, AdminRole, AdminService, AdminUser } from "@/admin/api/types";
import type { Service } from "../api/listServices";
import {
  seedServices as seedAdminServices,
  seedGroups,
  seedRoles,
  seedUsers,
} from "./admin/fixtures";
import { type AccessRequest, seedAccessRequests, seedCategories, seedServices } from "./fixtures";

type AdminStore = {
  users: AdminUser[];
  groups: AdminGroup[];
  roles: AdminRole[];
  services: AdminService[];
};

type Store = {
  services: Service[];
  accessRequests: AccessRequest[];
  categories: Record<string, string>;
  admin: AdminStore;
};

function snapshot(): Store {
  return {
    services: seedServices.map((s) => ({ ...s, category: [...s.category] })),
    accessRequests: seedAccessRequests.map((r) => ({ ...r })),
    categories: { ...seedCategories },
    admin: {
      users: seedUsers.map((u) => ({ ...u, groups: [...u.groups], roles: [...u.roles] })),
      groups: seedGroups.map((g) => ({ ...g, roles: [...g.roles] })),
      roles: seedRoles.map((r) => ({ ...r })),
      services: seedAdminServices.map((s) => ({ ...s, requiredGroups: [...s.requiredGroups] })),
    },
  };
}

export const store: Store = snapshot();

export function resetStore(): void {
  const fresh = snapshot();
  store.services = fresh.services;
  store.accessRequests = fresh.accessRequests;
  store.categories = fresh.categories;
  store.admin = fresh.admin;
}
