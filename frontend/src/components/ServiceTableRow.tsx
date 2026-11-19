import { ExternalLink } from "lucide-react";
import type { Service } from "../api/listServices";
import { cn } from "../lib/utils";
import { CategoryBadges } from "./CategoryBadge";
import { PinButton } from "./PinButton";
import { ServiceIcon } from "./ServiceIcon";
import { StatusBadge } from "./StatusBadge";
import { TableCell, TableRow } from "./ui/table";

type ServiceTableRowProps = {
  service: Service;
  onTogglePin?: (serviceId: string, nextPinned: boolean) => void;
};

export function ServiceTableRow({ service, onTogglePin }: ServiceTableRowProps) {
  const openService = () => {
    window.open(service.url, "_blank", "noopener,noreferrer");
  };

  return (
    <TableRow
      className="group/row cursor-pointer hover:bg-table-row-hover-background"
      onClick={openService}
    >
      <TableCell className="h-14 px-3 py-0">
        <div className="flex justify-end">
          <PinButton
            pinned={service.pinned}
            className={cn(
              !service.pinned &&
                "pointer-fine:opacity-0 pointer-fine:group-focus-within/row:opacity-100 pointer-fine:group-hover/row:opacity-100",
            )}
            onToggle={(nextPinned) => onTogglePin?.(service.id, nextPinned)}
          />
        </div>
      </TableCell>

      <TableCell className="h-14 p-2">
        <div className="flex min-w-0 items-center gap-2">
          <ServiceIcon
            name={service.name}
            image={service.image}
            imageLight={service.imageLight}
            imageDark={service.imageDark}
            variant="table"
          />

          <div className="min-w-0 flex-1 text-sm leading-5">
            <div className="flex min-w-0 items-center gap-1.5">
              <a
                href={service.url}
                target="_blank"
                rel="noreferrer"
                aria-label={`${service.name} (opens in a new tab)`}
                className="truncate rounded-sm font-medium text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring"
                onClick={(event) => event.stopPropagation()}
              >
                {service.name}
              </a>
              <ExternalLink
                aria-hidden="true"
                strokeWidth={1.5}
                className="size-3.5 shrink-0 text-muted-foreground-strong"
              />
            </div>
            <div
              className="truncate text-muted-foreground group-hover/row:text-muted-foreground-strong"
              title={service.description}
            >
              {service.description}
            </div>
          </div>
        </div>
      </TableCell>

      <TableCell className="h-14 px-3 whitespace-normal">
        <div className="flex min-w-0 flex-wrap gap-1">
          <CategoryBadges
            categories={service.category}
            badgeClassName="max-w-full whitespace-normal break-words"
          />
        </div>
      </TableCell>

      <TableCell className="h-14 px-3">
        <StatusBadge status={service.status} />
      </TableCell>
    </TableRow>
  );
}
