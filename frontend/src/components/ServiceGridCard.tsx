import { ExternalLink } from "lucide-react";
import type { Service } from "../api/listServices";
import { cn } from "../lib/utils";
import { CategoryBadges } from "./CategoryBadge";
import { PinButton } from "./PinButton";
import { ServiceIcon } from "./ServiceIcon";
import { StatusBadge } from "./StatusBadge";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "./ui/card";

type ServiceGridCardProps = {
  service: Service;
  /** Omit to render the card without a pin control. */
  onTogglePin?: (serviceId: string, nextPinned: boolean) => void | Promise<void>;
};

/** Service card; the whole card opens the service in a new tab. */
export function ServiceGridCard({ service, onTogglePin }: ServiceGridCardProps) {
  return (
    <div className="group/service relative h-full rounded-md">
      <a
        href={service.url}
        target="_blank"
        rel="noreferrer"
        aria-label={`${service.name} (opens in a new tab)`}
        className="peer absolute inset-0 z-10 rounded-md no-underline! outline-none"
      >
        <span className="sr-only">Open {service.name}</span>
      </a>
      <Card
        size="sm"
        className="h-full gap-3 rounded-md p-4 shadow-none peer-focus-visible:ring-2 peer-focus-visible:ring-ring peer-focus-visible:ring-inset group-hover/service:border-border-strong group-hover/service:bg-background group-hover/service:shadow-[0_4px_6px_rgb(0_0_0/0.08)] motion-safe:transition-[background-color,border-color,box-shadow] motion-safe:duration-(--duration-fast)"
      >
        <CardHeader className="flex items-center gap-3 px-0">
          <ServiceIcon
            name={service.name}
            image={service.image}
            imageLight={service.imageLight}
            imageDark={service.imageDark}
          />
          <div className="flex min-w-0 flex-1 items-center gap-1.5">
            <CardTitle className="truncate text-sm font-medium leading-5 text-foreground">
              {service.name}
            </CardTitle>
            <ExternalLink
              aria-hidden="true"
              strokeWidth={1.5}
              className="size-3.5 shrink-0 text-muted-foreground-strong"
            />
          </div>
          {onTogglePin && (
            <PinButton
              pinned={service.pinned}
              className={cn(
                "relative z-20",
                // Unpinned cards reveal the pin on hover or focus; touch screens always show it.
                !service.pinned &&
                  "pointer-fine:opacity-0 pointer-fine:group-focus-within/service:opacity-100 pointer-fine:group-hover/service:opacity-100",
              )}
              onToggle={(nextPinned) => void onTogglePin(service.id, nextPinned)}
            />
          )}
        </CardHeader>

        <CardContent className="px-0">
          <CardDescription
            className="line-clamp-2 h-10 text-muted-foreground-strong"
            title={service.description}
          >
            {service.description}
          </CardDescription>
        </CardContent>

        <CardFooter className="mt-auto flex-wrap gap-2 px-0">
          <CategoryBadges categories={service.category} />
          <StatusBadge status={service.status} />
        </CardFooter>
      </Card>
    </div>
  );
}
