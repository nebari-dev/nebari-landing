import type { Service } from "../api/listServices";
import { usesRicherLayout } from "../lib/services";
import { cn } from "../lib/utils";
import { ServicesSection } from "./ServicesSection";
import { SimpleServicesSection } from "./SimpleServicesSection";

type ContentProps = {
  services: Service[];
  /** First name for the greeting; the greeting drops the name when absent. */
  userName?: string;
  onTogglePin: (serviceId: string, nextPinned: boolean) => void | Promise<void>;
};

export function Content({ services, userName, onTogglePin }: ContentProps) {
  const richer = usesRicherLayout(services);

  return (
    <div className={cn("flex flex-col px-12 pt-8 pb-16", richer ? "gap-6" : "gap-8")}>
      <h1 className="text-3xl font-bold text-foreground">
        {userName ? `Welcome back, ${userName}` : "Welcome back"}
      </h1>
      {richer ? (
        <ServicesSection services={services} onTogglePin={onTogglePin} />
      ) : (
        <SimpleServicesSection services={services} />
      )}
    </div>
  );
}
