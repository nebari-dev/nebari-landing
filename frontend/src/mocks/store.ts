// Module-scoped, in-memory state used by the stateful MSW handlers. Tests
// reset between cases via resetStore(); the browser worker resets once on
// load. There is no persistence — refreshing the page rehydrates from the
// seed fixtures.

import type { Service } from "../api/listServices";
import { type AccessRequest, seedAccessRequests, seedCategories, seedServices } from "./fixtures";

type Store = {
  services: Service[];
  accessRequests: AccessRequest[];
  categories: Record<string, string>;
};

function snapshot(): Store {
  return {
    services: seedServices.map((s) => ({ ...s, category: [...s.category] })),
    accessRequests: seedAccessRequests.map((r) => ({ ...r })),
    categories: { ...seedCategories },
  };
}

export const store: Store = snapshot();

export function resetStore(): void {
  const fresh = snapshot();
  store.services = fresh.services;
  store.accessRequests = fresh.accessRequests;
  store.categories = fresh.categories;
}
