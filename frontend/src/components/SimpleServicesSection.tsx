import { LayoutGrid } from "lucide-react";
import type { Service } from "../api/listServices";
import { DEFAULT_SORT, sortServices } from "../lib/services";
import { ServicesEmptyState } from "./ServicesEmptyState";
import { ServicesGrid } from "./ServicesGrid";

type SimpleServicesSectionProps = {
  services: Service[];
  /** Shows skeleton cards while the services load. */
  loading?: boolean;
};

/**
 * Layout for fewer than 6 services: one grid in the operator's priority order
 * (the richer layout's default sort), without toolbar, sections, or pinning.
 */
export function SimpleServicesSection({ services, loading = false }: SimpleServicesSectionProps) {
  if (!loading && services.length === 0) {
    return (
      <ServicesEmptyState
        icon={LayoutGrid}
        title="No services yet"
        description="Your admin hasn't added any services to this deployment. Once they do, they'll show up here."
      />
    );
  }

  return (
    <ServicesGrid
      services={sortServices(services, DEFAULT_SORT)}
      emptyMessage="No services found."
      skeletonCount={loading ? 5 : undefined}
    />
  );
}
