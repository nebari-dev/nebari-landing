import type { Service } from "../api/listServices";
import { ServiceCardSkeleton } from "./ServiceCardSkeleton";
import { ServiceGridCard } from "./ServiceGridCard";

type ServicesGridProps = {
  services: Service[];
  /** Shown when `services` is empty. */
  emptyMessage?: string;
  /** Omit to render cards without a pin control. */
  onTogglePin?: (serviceId: string, nextPinned: boolean) => void | Promise<void>;
  /** Renders this many skeleton cards instead of the services. */
  skeletonCount?: number;
};

const GRID_CLASS = "grid gap-4 sm:grid-cols-2 lg:grid-cols-3";

export function ServicesGrid({
  services,
  emptyMessage,
  onTogglePin,
  skeletonCount,
}: ServicesGridProps) {
  if (skeletonCount) {
    return (
      <div className={GRID_CLASS}>
        {Array.from({ length: skeletonCount }, (_, index) => (
          <ServiceCardSkeleton key={index} />
        ))}
      </div>
    );
  }

  if (services.length === 0) {
    return emptyMessage ? <p className="text-sm text-muted-foreground">{emptyMessage}</p> : null;
  }

  return (
    <div className={GRID_CLASS}>
      {services.map((service) => (
        <ServiceGridCard key={service.id} service={service} onTogglePin={onTogglePin} />
      ))}
    </div>
  );
}
