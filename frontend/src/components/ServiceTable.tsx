import { PinIcon, UnpinIcon } from "./PinIcon"
import type { Service } from "../api/listServices"
import { StatusBadge } from "./StatusBadge"
import { Button } from "../components/ui/button"
import { ServiceIcon } from "./ServiceIcon"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "../components/ui/table"

type ServicesTableProps = {
  services: Service[]
  onTogglePin?: (serviceId: string, nextPinned: boolean) => void
}

export function ServicesTable({
  services,
  onTogglePin,
}: ServicesTableProps) {
  return (
    <div className="overflow-hidden rounded-xl border border-border bg-background transition-none">
      <Table>
        <TableHeader>
          <TableRow className="h-[54px] border-b border-border transition-none">
            <TableHead className="px-5 py-4 text-[13px] font-semibold uppercase tracking-[0.05em] text-(--text-secondary)">
              Service
            </TableHead>
            <TableHead className="px-5 py-4 text-[13px] font-semibold uppercase tracking-[0.05em] text-(--text-secondary)">
              Category
            </TableHead>
            <TableHead className="px-5 py-4 text-[13px] font-semibold uppercase tracking-[0.05em] text-(--text-secondary)">
              Status
            </TableHead>
            <TableHead className="px-5 py-4 text-right text-[13px] font-semibold uppercase tracking-[0.05em] text-(--text-secondary)">
              Actions
            </TableHead>
          </TableRow>
        </TableHeader>

        <TableBody>
          {services.map((service) => (
            <ServiceRow
              key={service.id}
              service={service}
              onTogglePin={onTogglePin}
            />
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function ServiceRow({
  service,
  onTogglePin,
}: {
  service: Service
  onTogglePin?: (serviceId: string, nextPinned: boolean) => void
}) {
  const openService = () => {
    window.open(service.url, "_blank", "noopener,noreferrer")
  }

  const handleRowKeyDown = (event: React.KeyboardEvent<HTMLTableRowElement>) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault()
      openService()
    }
  }

  return (
    <TableRow
      tabIndex={0}
      role="link"
      aria-label={`${service.name} (opens in a new tab)`}
      className="h-[77px] cursor-pointer border-b border-border transition-none focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
      onClick={openService}
      onKeyDown={handleRowKeyDown}
    >
      <TableCell className="px-5 py-4 align-middle">
        <div className="flex items-start gap-4">
          <ServiceIcon imageUrl={service.image} />

          <div className="min-w-0">
            <p className="text-sm font-semibold leading-5 text-foreground">
              {service.name}
            </p>
            <p className="text-(--text-secondary) text-sm leading-5">
              {service.description}
            </p>
          </div>
        </div>
      </TableCell>

      <TableCell className="px-5 py-4 align-middle">
        <div className="flex flex-wrap gap-2">
          {service.category.map((item) => (
            <span
              key={item}
              className="inline-flex items-center rounded-sm bg-accent px-2 py-0.5 text-xs capitalize text-(--pill-category-fg)"
            >
              {item}
            </span>
          ))}
        </div>
      </TableCell>

      <TableCell className="px-5 py-4 align-middle">
        <StatusBadge status={service.status} />
      </TableCell>

      <TableCell className="px-5 py-4 text-right align-middle">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-8 w-8 transition-none"
          onClick={(event) => {
            event.stopPropagation()
            onTogglePin?.(service.id, !service.pinned)
          }}
          onKeyDown={(event) => {
            event.stopPropagation()
          }}
          title={service.pinned ? "Unpin service" : "Pin service"}
          aria-label={service.pinned ? "Unpin service" : "Pin service"}
        >
          {service.pinned
            ? <UnpinIcon className="h-4 w-4 text-primary" />
            : <PinIcon className="h-4 w-4 text-muted-foreground" />
          }
        </Button>
      </TableCell>
    </TableRow>
  )
}
