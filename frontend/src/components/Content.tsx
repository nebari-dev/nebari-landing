import type {
  Service
} from "../api/listServices"

import {
  ServicesAccordion
} from "./ServiceAccordion"

type ContentProps = {
  services: Service[]
  onTogglePin: (serviceId: string, nextPinned: boolean) => void | Promise<void>
}

export function Content({ services, onTogglePin }: ContentProps) {
  const pinnedServices = services.filter((service) => service.pinned)

  return (
    <div className="px-12 py-6">
      <ServicesAccordion
        pinnedServices={pinnedServices}
        services={services}
        onTogglePin={onTogglePin}
      />
    </div>
  )
}
