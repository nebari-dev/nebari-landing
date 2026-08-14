import type { Service } from "../api/listServices";
import { PinIcon, UnpinIcon } from "./PinIcon";
import { ServiceIcon } from "./ServiceIcon";
import { StatusBadge } from "./StatusBadge";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "./ui/card";

type ServiceGridCardProps = {
  service: Service;
  onTogglePin: (serviceId: string, nextPinned: boolean) => void | Promise<void>;
};

export function ServiceGridCard({ service, onTogglePin }: ServiceGridCardProps) {
  return (
    <div className="group/card relative h-56 overflow-visible rounded-md">
      <a
        href={service.url}
        target="_blank"
        rel="noreferrer"
        aria-label={`${service.name} (opens in a new tab)`}
        className="peer absolute inset-0 z-10 rounded-md outline-none"
      >
        <span className="sr-only">Open {service.name}</span>
      </a>
      <Card
        size="sm"
        className="h-full shadow-none peer-focus-visible:ring-2 peer-focus-visible:ring-ring peer-focus-visible:ring-inset group-hover/card:bg-black/[0.02] dark:group-hover/card:bg-white/[0.03]"
      >
        <CardHeader>
          <ServiceIcon
            image={service.image}
            imageLight={service.imageLight}
            imageDark={service.imageDark}
          />
          <CardAction>
            <StatusBadge status={service.status} />
          </CardAction>
        </CardHeader>

        <CardContent className="min-h-0">
          <CardTitle className="text-sm font-medium leading-5 text-foreground">
            {service.name}
          </CardTitle>
          <CardDescription className="mt-1 line-clamp-2" title={service.description}>
            {service.description}
          </CardDescription>
        </CardContent>

        <CardFooter className="relative mt-auto justify-between pt-(--card-spacing)">
          <div
            aria-hidden="true"
            className="absolute top-0 right-(--card-spacing) left-(--card-spacing) border-t"
          />
          <div className="flex min-w-0 flex-wrap gap-2 overflow-hidden">
            {service.category.map((item) => (
              <Badge
                key={item}
                variant="secondary"
                className="rounded-sm bg-category-badge-background text-category-badge-foreground capitalize"
              >
                {item}
              </Badge>
            ))}
          </div>

          <Button
            variant="ghost"
            size="icon"
            className="relative z-20 focus-visible:ring-offset-0"
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              void onTogglePin(service.id, !service.pinned);
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
        </CardFooter>
      </Card>
    </div>
  );
}
