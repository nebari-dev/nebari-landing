import type { Service } from "../api/listServices"

type ServiceImageFields = Pick<Service, "image" | "iconLight" | "iconDark">

export function getServiceImageUrl(
  service: ServiceImageFields,
  isDarkMode: boolean
) {
  return (isDarkMode ? service.iconDark : service.iconLight) || service.image
}
