import type { Service } from "../api/listServices";
import { ServiceGridCard } from "./ServiceGridCard";

type ServicesGridProps = {
  services: Service[];
  /** Shown when `services` is empty. */
  emptyMessage?: string;
  /** Omit to render cards without a pin control. */
  onTogglePin?: (serviceId: string, nextPinned: boolean) => void | Promise<void>;
};

export function ServicesGrid({ services, emptyMessage, onTogglePin }: ServicesGridProps) {
  if (services.length === 0) {
    return emptyMessage ? <p className="text-sm text-muted-foreground">{emptyMessage}</p> : null;
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {services.map((service) => (
        <ServiceGridCard key={service.id} service={service} onTogglePin={onTogglePin} />
      ))}
    </div>
  );
}
