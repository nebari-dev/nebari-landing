import { useMemo, useState } from "react";
import type { Service } from "../api/listServices";
import {
  DEFAULT_SORT,
  filterServices,
  getCategories,
  type ServiceSort,
  selectsAllCategories,
  sortServices,
} from "../lib/services";
import { ServicesAccordion } from "./ServiceAccordion";
import { ServicesToolbar } from "./ServicesToolbar";
import { ServicesTable } from "./ServiceTable";
import type { ServicesView } from "./ViewToggle";

type ServicesSectionProps = {
  services: Service[];
  onTogglePin: (serviceId: string, nextPinned: boolean) => void | Promise<void>;
};

/** Richer layout: toolbar above either the sectioned grid or the table. */
export function ServicesSection({ services, onTogglePin }: ServicesSectionProps) {
  const [query, setQuery] = useState("");
  const [selectedCategories, setSelectedCategories] = useState<string[]>([]);
  const [sort, setSort] = useState<ServiceSort>(DEFAULT_SORT);
  const [view, setView] = useState<ServicesView>("grid");

  const categories = useMemo(() => getCategories(services), [services]);

  const visibleServices = useMemo(() => {
    const activeCategories = selectsAllCategories(categories, selectedCategories)
      ? []
      : selectedCategories;
    return sortServices(filterServices(services, query, activeCategories), sort);
  }, [services, categories, query, selectedCategories, sort]);

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
        onViewChange={setView}
      />

      {view === "grid" ? (
        <ServicesAccordion
          pinnedServices={visibleServices.filter((service) => service.pinned)}
          otherServices={visibleServices.filter((service) => !service.pinned)}
          onTogglePin={onTogglePin}
        />
      ) : visibleServices.length === 0 ? (
        <p className="text-sm text-muted-foreground">No services found.</p>
      ) : (
        <ServicesTable
          services={visibleServices}
          sort={sort}
          onSortChange={setSort}
          onTogglePin={onTogglePin}
        />
      )}
    </div>
  );
}
