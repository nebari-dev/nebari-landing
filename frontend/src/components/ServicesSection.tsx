import { SearchX } from "lucide-react";
import { useMemo, useState } from "react";
import type { Service } from "../api/listServices";
import { readServicesView, writeServicesView } from "../lib/preferences";
import {
  DEFAULT_SORT,
  filterServices,
  getCategories,
  type ServiceSort,
  sortServices,
} from "../lib/services";
import { ServicesAccordion } from "./ServiceAccordion";
import { ServicesEmptyState } from "./ServicesEmptyState";
import { ServicesToolbar } from "./ServicesToolbar";
import { ServicesTable } from "./ServiceTable";
import { Button } from "./ui/button";
import type { ServicesView } from "./ViewToggle";

type ServicesSectionProps = {
  services: Service[];
  onTogglePin: (serviceId: string, nextPinned: boolean) => void | Promise<void>;
  /** Shows skeletons in the current view while the services load. */
  loading?: boolean;
};

/** Richer layout: toolbar above either the sectioned grid or the table. */
export function ServicesSection({ services, onTogglePin, loading = false }: ServicesSectionProps) {
  const [query, setQuery] = useState("");
  const [selectedCategories, setSelectedCategories] = useState<string[]>([]);
  const [sort, setSort] = useState<ServiceSort>(DEFAULT_SORT);
  const [view, setView] = useState<ServicesView>(() => readServicesView() ?? "grid");

  const categories = useMemo(() => getCategories(services), [services]);

  const visibleServices = useMemo(
    () => sortServices(filterServices(services, query, selectedCategories), sort),
    [services, query, selectedCategories, sort],
  );

  const changeView = (nextView: ServicesView) => {
    setView(nextView);
    writeServicesView(nextView);
  };

  const clearSearch = () => {
    setQuery("");
    setSelectedCategories([]);
  };

  const trimmedQuery = query.trim();

  return (
    <div className="flex flex-col gap-6">
      <ServicesToolbar
        query={query}
        onQueryChange={setQuery}
        categories={categories}
        selectedCategories={selectedCategories}
        onSelectedCategoriesChange={setSelectedCategories}
        sort={sort}
        onSortChange={setSort}
        view={view}
        onViewChange={changeView}
      />

      {!loading && visibleServices.length === 0 ? (
        <ServicesEmptyState
          role="status"
          icon={SearchX}
          title={
            trimmedQuery
              ? `No services match "${trimmedQuery}"`
              : "No services match the selected categories"
          }
          description="Check the spelling, or clear the search and filters to see everything."
          action={
            <Button variant="outline" size="sm" onClick={clearSearch}>
              {trimmedQuery ? "Clear search" : "Clear filters"}
            </Button>
          }
        />
      ) : view === "grid" ? (
        <ServicesAccordion
          pinnedServices={visibleServices.filter((service) => service.pinned)}
          otherServices={visibleServices.filter((service) => !service.pinned)}
          onTogglePin={onTogglePin}
          loading={loading}
        />
      ) : (
        <ServicesTable
          services={visibleServices}
          sort={sort}
          onSortChange={setSort}
          onTogglePin={onTogglePin}
          loading={loading}
        />
      )}
    </div>
  );
}
