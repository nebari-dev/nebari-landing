import type { Service } from "../api/listServices";
import { ServicesGrid } from "./ServicesGrid";

type SimpleServicesSectionProps = {
  services: Service[];
};

/** Layout for fewer than 6 services: one grid in API order, without toolbar, sections, or pinning. */
export function SimpleServicesSection({ services }: SimpleServicesSectionProps) {
  return <ServicesGrid services={services} emptyMessage="No services found." />;
}
