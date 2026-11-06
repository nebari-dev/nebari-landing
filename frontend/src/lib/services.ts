import type { Service } from "../api/listServices";

export type SortKey = "name" | "category" | "status";
export type SortDirection = "asc" | "desc";
export type ServiceSort = { key: SortKey; direction: SortDirection };

export const DEFAULT_SORT: ServiceSort = { key: "name", direction: "asc" };

/** Fewer services than this get the simple layout: no toolbar, sections, or pinning. */
export const RICHER_LAYOUT_MIN_SERVICES = 6;

/** True when the user's full service list is long enough for the richer layout. */
export function usesRicherLayout(services: Service[]) {
  return services.length >= RICHER_LAYOUT_MIN_SERVICES;
}

const collator = new Intl.Collator(undefined, { sensitivity: "base", numeric: true });

/** Available services rank ahead of unknown ones, which rank ahead of unavailable ones. */
function statusRank(status: string) {
  const normalized = status.toLowerCase();
  if (normalized === "healthy") return 0;
  if (normalized === "unhealthy") return 2;
  return 1;
}

function compareBy(key: SortKey, a: Service, b: Service) {
  switch (key) {
    case "name":
      return collator.compare(a.name, b.name);
    case "category":
      return collator.compare(a.category[0] ?? "", b.category[0] ?? "");
    case "status":
      return statusRank(a.status) - statusRank(b.status);
  }
}

/** Sorts a copy of `services`; ties on category or status fall back to name A–Z. */
export function sortServices(services: Service[], { key, direction }: ServiceSort): Service[] {
  const sign = direction === "asc" ? 1 : -1;

  return [...services].sort(
    (a, b) => sign * compareBy(key, a, b) || collator.compare(a.name, b.name),
  );
}

/**
 * Keeps services that match the search query (name, description, or category)
 * and, when any categories are selected, carry at least one of them.
 */
export function filterServices(
  services: Service[],
  query: string,
  categories: string[],
): Service[] {
  const normalizedQuery = query.trim().toLowerCase();

  return services.filter((service) => {
    if (categories.length > 0 && !service.category.some((item) => categories.includes(item))) {
      return false;
    }
    if (!normalizedQuery) return true;

    return [service.name, service.description, ...service.category]
      .join(" ")
      .toLowerCase()
      .includes(normalizedQuery);
  });
}

/** Every category used by at least one service, A–Z. */
export function getCategories(services: Service[]): string[] {
  return [...new Set(services.flatMap((service) => service.category))].sort(collator.compare);
}

/** True when the selection covers every category, so it filters nothing. */
export function selectsAllCategories(categories: string[], selected: string[]): boolean {
  return categories.length > 1 && categories.every((category) => selected.includes(category));
}
