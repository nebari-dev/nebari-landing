import type { Service } from "../api/listServices"
import { ServiceGridCard } from "./ServiceGridCard"

type ServicesGridProps = {
  services: Service[]
  onTogglePin: (serviceId: string, nextPinned: boolean) => void | Promise<void>
}

export function ServicesGrid({ services, onTogglePin }: ServicesGridProps) {
  if (services.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        No services found.
      </p>
    )
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      {services.map((service) => (
        <ServiceGridCard
          key={service.id}
          service={service}
          onTogglePin={onTogglePin}
        />
      ))}
    </div>
  )
}
