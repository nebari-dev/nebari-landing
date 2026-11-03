import type { Service } from "../api/listServices";
import { Accordion, AccordionContent, AccordionItem } from "../components/ui/accordion";
import { ServiceSectionTrigger } from "./ServiceSectionTrigger";
import { ServicesGrid } from "./ServicesGrid";

type ServicesAccordionProps = {
  pinnedServices: Service[];
  /** Services that are not pinned; pinned services appear only in the Pinned section. */
  otherServices: Service[];
  onTogglePin: (serviceId: string, nextPinned: boolean) => void | Promise<void>;
  /** Shows skeleton cards, without counts, while the services load. */
  loading?: boolean;
  /** Opens the details drawer for a service. */
  onShowDetails?: (serviceId: string) => void;
};

/** Richer-layout grid view: collapsible Pinned and All services sections. */
export function ServicesAccordion({
  pinnedServices,
  otherServices,
  onTogglePin,
  loading = false,
  onShowDetails,
}: ServicesAccordionProps) {
  return (
    <Accordion multiple defaultValue={["pinned-services", "all-services"]} className="gap-6">
      <AccordionItem value="pinned-services" className="border-b-0">
        <ServiceSectionTrigger title="Pinned" count={loading ? undefined : pinnedServices.length} />
        <AccordionContent className="text-foreground" contentClassName="px-0 pt-3 pb-0">
          <ServicesGrid
            services={pinnedServices}
            emptyMessage="No pinned services yet."
            onTogglePin={onTogglePin}
            skeletonCount={loading ? 2 : undefined}
            onShowDetails={onShowDetails}
          />
        </AccordionContent>
      </AccordionItem>

      <AccordionItem value="all-services" className="border-b-0">
        <ServiceSectionTrigger
          title="All services"
          count={loading ? undefined : otherServices.length}
        />
        <AccordionContent className="text-foreground" contentClassName="px-0 pt-3 pb-0">
          <ServicesGrid
            services={otherServices}
            emptyMessage="No services found."
            onTogglePin={onTogglePin}
            skeletonCount={loading ? 6 : undefined}
            onShowDetails={onShowDetails}
          />
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  );
}
