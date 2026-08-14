import type { Service } from "../api/listServices";
import { Badge } from "../components/ui/badge";
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
    <Table
      aria-label="Services"
      className="min-w-140 table-fixed"
      scrollContainerClassName="focus-visible:ring-inset focus-visible:ring-offset-0"
    >
      <colgroup>
        <col className="w-[50%]" />
        <col className="w-[20%]" />
        <col className="w-[15%]" />
        <col className="w-[15%]" />
      </colgroup>

      <TableHeader className="[&_th]:bg-muted dark:[&_th]:bg-background">
        <TableRow>
          <TableHead scope="col">Service</TableHead>
          <TableHead scope="col">Category</TableHead>
          <TableHead scope="col">Status</TableHead>
          <TableHead className="text-right" scope="col">
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
      className="cursor-pointer outline-none hover:bg-table-row-hover-background focus-visible:bg-table-row-hover-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
      onClick={openService}
      onKeyDown={handleRowKeyDown}
    >
      <TableCell className="whitespace-normal">
        <div className="flex min-w-0 items-center gap-3">
          <ServiceIcon
            image={service.image}
            imageLight={service.imageLight}
            imageDark={service.imageDark}
          />

          <div className="min-w-0 flex-1">
            <div className="truncate text-sm font-medium leading-5 text-foreground">
              {service.name}
            </div>
            <div className="overflow-hidden text-muted-foreground text-sm leading-5 whitespace-normal break-words [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
              {service.description}
            </div>
          </div>
        </div>
      </TableCell>

      <TableCell className="whitespace-normal">
        <div className="flex min-w-0 flex-wrap gap-1">
          {service.category.map((item) => (
            <Badge
              key={item}
              variant="secondary"
              className="max-w-full rounded-sm bg-category-badge-background text-category-badge-foreground capitalize whitespace-normal break-words"
            >
              {item}
            </Badge>
          ))}
        </div>
      </TableCell>

      <TableCell>
        <div className="min-w-0">
          <StatusBadge status={service.status} />
        </div>
      </TableCell>

      <TableCell className="text-right">
        <Button
          variant="ghost"
          size="icon"
          className="focus-visible:ring-offset-0"
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
            <UnpinIcon className="text-primary" />
          ) : (
            <PinIcon className="text-muted-foreground" />
          )}
        </Button>
      </TableCell>
    </TableRow>
  );
}
