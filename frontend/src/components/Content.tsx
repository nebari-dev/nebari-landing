import { CircleAlert } from "lucide-react";
import { useEffect, useState } from "react";
import type { Service } from "../api/listServices";
import type { ServicesStatus } from "../hooks/useLaunchpadData";
import { readServicesLayout, writeServicesLayout } from "../lib/preferences";
import { usesRicherLayout } from "../lib/services";
import { cn } from "../lib/utils";
import { ServiceDetailsDrawer } from "./ServiceDetailsDrawer";
import { ServicesEmptyState } from "./ServicesEmptyState";
import { ServicesSection } from "./ServicesSection";
import { SimpleServicesSection } from "./SimpleServicesSection";
import { Button } from "./ui/button";

type ContentProps = {
  services: Service[];
  status: ServicesStatus;
  /** Requests the services again after an error. */
  onRetry: () => void;
  /** First name for the greeting; the greeting drops the name when absent. */
  userName?: string;
  onTogglePin: (serviceId: string, nextPinned: boolean) => void | Promise<void>;
};

export function Content({ services, status, onRetry, userName, onTogglePin }: ContentProps) {
  const [lastLayout] = useState(readServicesLayout);
  const loading = status === "loading";
  const richer = loading ? lastLayout === "richer" : usesRicherLayout(services);

  useEffect(() => {
    if (status === "ready") writeServicesLayout(richer ? "richer" : "simple");
  }, [status, richer]);

  const [detailsServiceId, setDetailsServiceId] = useState<string>();
  const [detailsOpen, setDetailsOpen] = useState(false);
  const detailsService = services.find((service) => service.id === detailsServiceId);

  const showDetails = (serviceId: string) => {
    setDetailsServiceId(serviceId);
    setDetailsOpen(true);
  };

  return (
    <div className={cn("flex flex-col px-12 pt-8 pb-16", richer ? "gap-6" : "gap-8")}>
      <h1 className="text-3xl font-bold text-foreground">
        {userName ? `Welcome back, ${userName}` : "Welcome back"}
      </h1>
      {loading && (
        <p role="status" className="sr-only">
          Loading services…
        </p>
      )}
      {status === "error" ? (
        <ServicesEmptyState
          role="alert"
          tone="destructive"
          icon={CircleAlert}
          title="Couldn't load services"
          description="Something went wrong reaching the cluster. Your services are probably fine. Try again in a moment."
          action={
            <Button variant="outline" size="sm" onClick={onRetry}>
              Try again
            </Button>
          }
        />
      ) : richer ? (
        <ServicesSection
          services={services}
          onTogglePin={onTogglePin}
          onShowDetails={showDetails}
          loading={loading}
        />
      ) : (
        <SimpleServicesSection services={services} onShowDetails={showDetails} loading={loading} />
      )}
      <ServiceDetailsDrawer
        service={detailsService}
        open={detailsOpen}
        onOpenChange={setDetailsOpen}
      />
    </div>
  );
}
