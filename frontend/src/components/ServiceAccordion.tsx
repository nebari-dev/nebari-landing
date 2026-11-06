import type { Service } from "../api/listServices";
import { Accordion, AccordionContent, AccordionItem } from "../components/ui/accordion";
import { ServiceSectionTrigger } from "./ServiceSectionTrigger";
import { ServicesGrid } from "./ServicesGrid";

type ServicesAccordionProps = {
  pinnedServices: Service[];
  /** Services that are not pinned; pinned services appear only in the Pinned section. */
  otherServices: Service[];
  onTogglePin: (serviceId: string, nextPinned: boolean) => void | Promise<void>;
};

/** Richer-layout grid view: collapsible Pinned and All services sections; Pinned is omitted when empty. */
export function ServicesAccordion({
  pinnedServices,
  otherServices,
  onTogglePin,
}: ServicesAccordionProps) {
  return (
    <Accordion multiple defaultValue={["pinned-services", "all-services"]} className="gap-6">
      {pinnedServices.length > 0 && (
        <AccordionItem value="pinned-services" className="border-b-0">
          <ServiceSectionTrigger title="Pinned" count={pinnedServices.length} />
          <AccordionContent className="text-foreground" contentClassName="px-0 pt-3 pb-0">
            <ServicesGrid services={pinnedServices} onTogglePin={onTogglePin} />
          </AccordionContent>
        </AccordionItem>
      )}

      {(otherServices.length > 0 || pinnedServices.length === 0) && (
        <AccordionItem value="all-services" className="border-b-0">
          <ServiceSectionTrigger title="All services" count={otherServices.length} />
          <AccordionContent className="text-foreground" contentClassName="px-0 pt-3 pb-0">
            <ServicesGrid
              services={otherServices}
              emptyMessage="No services found."
              onTogglePin={onTogglePin}
            />
          </AccordionContent>
        </AccordionItem>
      )}
    </Accordion>
  );
}
