import type { Service } from "../api/listServices"
import { Button } from "./ui/button"
import { Card, CardContent } from "./ui/card"
import { StatusBadge } from "./StatusBadge"
import { ServiceIcon } from "./ServiceIcon"
import pinIcon from "../assets/pin.svg"
import unpinIcon from "../assets/unpin.svg"

type ServiceGridCardProps = {
  service: Service
  onTogglePin: (serviceId: string, nextPinned: boolean) => void | Promise<void>
}

export function ServiceGridCard({
  service,
  onTogglePin,
}: ServiceGridCardProps) {
  return (
    <a
      href={service.url}
      target="_blank"
      rel="noreferrer"
      className="block overflow-visible focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
    >
      <Card className="overflow-hidden border border-border bg-background shadow-none transition-none hover:bg-black/[0.02] dark:hover:bg-white/[0.03]">
        <CardContent className="p-4">
          <div className="flex items-start justify-between gap-3">
            <ServiceIcon imageUrl={service.image} />

            <StatusBadge status={service.status} />
          </div>

          <div className="mt-4 block">
            <p className="text-base font-bold text-foreground">{service.name}</p>
            <p className="mt-1 text-sm text-muted-foreground">
              {service.description}
            </p>
          </div>

          <div className="my-4 border-t" />

          <div className="flex items-center justify-between gap-3">
            <div className="flex flex-wrap gap-2">
              {service.category.map((item) => (
                <span
                  key={item}
                  className="inline-flex items-center rounded-sm bg-muted px-2 py-0.5 text-xs capitalize text-muted-foreground"
                >
                  {item}
                </span>
              ))}
            </div>

            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-8 w-8 shrink-0"
              onClick={(event) => {
                event.preventDefault()
                event.stopPropagation()
                void onTogglePin(service.id, !service.pinned)
              }}
              title={service.pinned ? "Unpin service" : "Pin service"}
              aria-label={service.pinned ? "Unpin service" : "Pin service"}
            >
              <img
                src={service.pinned ? unpinIcon : pinIcon}
                alt=""
                aria-hidden="true"
                className="h-4 w-4 object-contain"
              />
            </Button>
          </div>
        </CardContent>
      </Card>
    </a>
  )
}
