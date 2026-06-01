// Module-scoped, in-memory state used by the stateful MSW handlers. Tests
// reset between cases via resetStore(); the browser worker resets once on
// load. There is no persistence — refreshing the page rehydrates from the
// seed fixtures.

import type { Service } from "../api/listServices";
import type { Notification } from "../api/notifications";
import {
  type AccessRequest,
  seedAccessRequests,
  seedCategories,
  seedNotifications,
  seedServices,
} from "./fixtures";

type Store = {
  services: Service[];
  notifications: Notification[];
  accessRequests: AccessRequest[];
  categories: Record<string, string>;
};

function snapshot(): Store {
  return {
    services: seedServices.map((s) => ({ ...s, category: [...s.category] })),
    notifications: seedNotifications.map((n) => ({ ...n })),
    accessRequests: seedAccessRequests.map((r) => ({ ...r })),
    categories: { ...seedCategories },
  };
}

export const store: Store = snapshot();

export function resetStore(): void {
  const fresh = snapshot();
  store.services = fresh.services;
  store.notifications = fresh.notifications;
  store.accessRequests = fresh.accessRequests;
  store.categories = fresh.categories;
}
