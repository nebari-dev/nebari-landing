import { Card, CardContent } from "../components/ui/card"
import { StatusBadge } from "./StatusBadge"
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
      className="block rounded-[4px] focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
    >
      <Card className="h-24 rounded-[4px] border border-border bg-card text-card-foreground shadow-none transition-none hover:bg-black/[0.02] dark:hover:bg-white/[0.03]">
        <CardContent className="flex h-full items-center gap-4 p-6">
          <div className="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-[10px] bg-muted">
            {service.image ? (
              <img
                src={service.image}
                alt={service.name}
                className="h-8 w-8 object-contain"
              />
            ) : (
              <span className="text-xs font-semibold">
                {service.name.slice(0, 2)}
              </span>
            )}
          </div>

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
