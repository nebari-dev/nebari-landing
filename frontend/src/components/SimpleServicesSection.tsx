import { LayoutGrid } from "lucide-react";
import type { Service } from "../api/listServices";
import { ServicesEmptyState } from "./ServicesEmptyState";
import { ServicesGrid } from "./ServicesGrid";

type SimpleServicesSectionProps = {
  services: Service[];
  /** Shows skeleton cards while the services load. */
  loading?: boolean;
  /** Opens the details drawer for a service. */
  onShowDetails?: (serviceId: string) => void;
};

/** Layout for fewer than 6 services: one grid in API order, without toolbar, sections, or pinning. */
export function SimpleServicesSection({
  services,
  onShowDetails,
  loading = false,
}: SimpleServicesSectionProps) {
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
      services={services}
      emptyMessage="No services found."
      skeletonCount={loading ? 5 : undefined}
      onShowDetails={onShowDetails}
    />
  );
}
