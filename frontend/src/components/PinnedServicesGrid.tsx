import type { Service } from "../api/listServices"
import { PinnedServiceCard } from "./PinnedServiceCard"

type PinnedServicesGridProps = {
  services: Service[]
}

export function PinnedServicesGrid({ services }: PinnedServicesGridProps) {
  if (services.length === 0) {
    return (
      <p className="text-sm text-(--text-secondary)">
        No pinned services yet.
      </p>
    )
  }

  return (
    <div className="grid overflow-visible gap-4 px-1 sm:grid-cols-2 xl:grid-cols-4">
      {services.map((service) => (
        <PinnedServiceCard key={service.id} service={service} />
      ))}
    </div>
  )
}
