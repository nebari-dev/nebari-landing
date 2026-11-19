import type { ServiceSort } from "../lib/services";
import { CategoryFilter } from "./CategoryFilter";
import { ServiceSearch } from "./ServiceSearch";
import { SortSelect } from "./SortSelect";
import { type ServicesView, ViewToggle } from "./ViewToggle";

type ServicesToolbarProps = {
  query: string;
  onQueryChange: (query: string) => void;
  categories: string[];
  selectedCategories: string[];
  onSelectedCategoriesChange: (categories: string[]) => void;
  sort: ServiceSort;
  onSortChange: (sort: ServiceSort) => void;
  view: ServicesView;
  onViewChange: (view: ServicesView) => void;
};

/** Search, category filter, sort, and view toggle for the richer layout. */
export function ServicesToolbar({
  query,
  onQueryChange,
  categories,
  selectedCategories,
  onSelectedCategoriesChange,
  sort,
  onSortChange,
  view,
  onViewChange,
}: ServicesToolbarProps) {
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
      <ServiceSearch value={query} onChange={onQueryChange} className="w-full sm:w-70" />

      <div className="order-last flex w-full min-w-0 flex-col gap-2 sm:flex-row sm:items-center lg:order-none lg:w-auto lg:flex-1">
        <CategoryFilter
          categories={categories}
          value={selectedCategories}
          onChange={onSelectedCategoriesChange}
          className="sm:w-auto sm:max-w-80 sm:min-w-55"
        />
        <SortSelect sort={sort} onChange={onSortChange} className="sm:w-auto sm:min-w-39" />
      </div>

      <div className="ml-auto">
        <ViewToggle view={view} onChange={onViewChange} />
      </div>
    </div>
  );
}
