import type { Service } from "../api/listServices";
import { Button } from "../components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "../components/ui/table";
import { PinIcon, UnpinIcon } from "./PinIcon";
import { ServiceIcon } from "./ServiceIcon";
import { StatusBadge } from "./StatusBadge";

type ServicesTableProps = {
  services: Service[];
  onTogglePin?: (serviceId: string, nextPinned: boolean) => void;
};

export function ServicesTable({ services, onTogglePin }: ServicesTableProps) {
  return (
    <div className="overflow-hidden rounded-xl border border-border bg-background transition-none">
      <Table className="min-w-140 table-fixed">
        <colgroup>
          <col className="w-[50%]" />
          <col className="w-[20%]" />
          <col className="w-[15%]" />
          <col className="w-[15%]" />
        </colgroup>

        <TableHeader>
          <TableRow className="h-13.5 border-b border-border transition-none">
            <TableHead className="w-[50%] py-4 pr-2 pl-5 text-[13px] font-semibold uppercase tracking-[0.05em] text-(--text-secondary)">
              Service
            </TableHead>
            <TableHead className="w-[20%] py-4 text-[13px] font-semibold uppercase tracking-[0.05em] text-(--text-secondary)">
              Category
            </TableHead>
            <TableHead className="w-[20%] py-4 text-[13px] font-semibold uppercase tracking-[0.05em] text-(--text-secondary)">
              Status
            </TableHead>
            <TableHead className="w-[10%] pr-5 py-4 text-right text-[12px] font-semibold uppercase tracking-[0.02em] text-(--text-secondary) md:text-[13px] md:tracking-[0.05em]">
              Actions
            </TableHead>
          </TableRow>
        </TableHeader>

        <TableBody>
          {services.map((service) => (
            <ServiceRow key={service.id} service={service} onTogglePin={onTogglePin} />
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function ServiceRow({
  service,
  onTogglePin,
}: {
  service: Service;
  onTogglePin?: (serviceId: string, nextPinned: boolean) => void;
}) {
  const openService = () => {
    window.open(service.url, "_blank", "noopener,noreferrer");
  };

  const handleRowKeyDown = (event: React.KeyboardEvent<HTMLTableRowElement>) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      openService();
    }
  };

  return (
    <TableRow
      tabIndex={0}
      role="link"
      aria-label={`${service.name} (opens in a new tab)`}
      className="h-[77px] cursor-pointer border-b border-border transition-none focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
      onClick={openService}
      onKeyDown={handleRowKeyDown}
    >
      <TableCell className="py-4 pr-2 pl-5 align-middle whitespace-normal">
        <div className="flex min-w-0 items-start gap-3">
          <ServiceIcon imageUrl={service.image} />

          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-semibold leading-5 text-foreground">
              {service.name}
            </p>
            <p className="overflow-hidden text-(--text-secondary) text-sm leading-5 whitespace-normal break-words [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
              {service.description}
            </p>
          </div>
        </div>
      </TableCell>

      <TableCell className="px-2 py-4 align-middle whitespace-normal">
        <div className="flex min-w-0 flex-wrap gap-1">
          {service.category.map((item) => (
            <span
              key={item}
              className="inline-flex max-w-full items-center rounded-sm bg-accent px-1.5 py-0.5 text-xs capitalize text-(--pill-category-fg) whitespace-normal break-words"
            >
              {item}
            </span>
          ))}
        </div>
      </TableCell>

      <TableCell className="px-2 py-4 align-middle">
        <div className="min-w-0">
          <StatusBadge status={service.status} />
        </div>
      </TableCell>

      <TableCell className="py-4 pr-5 pl-2 text-right align-middle">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-8 w-8 transition-none"
          onClick={(event) => {
            event.stopPropagation();
            onTogglePin?.(service.id, !service.pinned);
          }}
          onKeyDown={(event) => {
            event.stopPropagation();
          }}
          title={service.pinned ? "Unpin service" : "Pin service"}
          aria-label={service.pinned ? "Unpin service" : "Pin service"}
        >
          {service.pinned ? (
            <UnpinIcon className="h-4 w-4 text-primary" />
          ) : (
            <PinIcon className="h-4 w-4 text-muted-foreground" />
          )}
        </Button>
      </TableCell>
    </TableRow>
  );
}
