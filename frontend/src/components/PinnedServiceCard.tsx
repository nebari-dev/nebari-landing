import { Card, CardContent } from "../components/ui/card"
import { StatusBadge } from "./StatusBadge"
import { ServiceIcon } from "./ServiceIcon"
import type { Service } from "../api/listServices"

type PinnedServiceCardProps = {
  service: Service
}

export function PinnedServiceCard({ service }: PinnedServiceCardProps) {
  return (
    <a
      href={service.url}
      target="_blank"
      rel="noreferrer"
      className="block focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
    >
      <Card className="h-24 border border-border bg-card text-card-foreground shadow-none transition-none hover:bg-black/[0.02] dark:hover:bg-white/[0.03]">
        <CardContent className="flex h-full items-center gap-4 p-6">
          <ServiceIcon imageUrl={service.image} />

          <div className="min-w-0">
            <p className="truncate text-[16px] font-bold leading-none text-foreground">
              {service.name}
            </p>
            <div className="mt-2">
              <StatusBadge status={service.status} />
            </div>
          </div>
        </CardContent>
      </Card>
    </a>
  )
}
