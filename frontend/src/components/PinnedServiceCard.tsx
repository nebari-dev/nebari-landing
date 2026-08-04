import type { Service } from "../api/listServices";
import { Card, CardContent } from "../components/ui/card";
import { ServiceIcon } from "./ServiceIcon";
import { StatusBadge } from "./StatusBadge";

type PinnedServiceCardProps = {
  service: Service;
};

export function PinnedServiceCard({ service }: PinnedServiceCardProps) {
  return (
    <a
      href={service.url}
      target="_blank"
      rel="noreferrer"
      className="group/card-link block rounded-md outline-none"
    >
      <Card className="h-24 border border-border bg-card text-card-foreground shadow-none transition-none group-focus-visible/card-link:ring-2 group-focus-visible/card-link:ring-ring group-focus-visible/card-link:ring-inset hover:bg-black/[0.02] dark:hover:bg-white/[0.03]">
        <CardContent className="flex h-full items-center gap-4 p-6">
          <ServiceIcon
            image={service.image}
            imageLight={service.imageLight}
            imageDark={service.imageDark}
          />

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
  );
}
