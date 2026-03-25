import type { Service } from "./listServices"
import type { BackendSocketService } from "./servicesSocket"

export function mapService(service: BackendSocketService): Service {
  return {
    id: service.uid,
    name: service.displayName,
    status:
      service.health.status.toLowerCase() === "healthy"
        ? "Healthy"
        : service.health.status.toLowerCase() === "unhealthy"
          ? "Unhealthy"
          : "Unknown",
    description: service.description,
    category: service.category ? [service.category] : [],
    pinned: false,
    image: service.icon,
    url: service.url,
  }
}
