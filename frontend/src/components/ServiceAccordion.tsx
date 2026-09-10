import type { Service } from "../api/listServices";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "../components/ui/accordion";
import { PinnedServicesGrid } from "./PinnedServicesGrid";
import { ServicesSection } from "./ServicesSection";

type ServicesAccordionProps = {
  pinnedServices: Service[];
  services: Service[];
  onTogglePin: (serviceId: string, nextPinned: boolean) => void | Promise<void>;
};

export function ServicesAccordion({
  pinnedServices,
  services,
  onTogglePin,
}: ServicesAccordionProps) {
  return (
    <Accordion multiple defaultValue={["pinned-services", "all-services"]}>
      <AccordionItem value="pinned-services" className="border-b-0">
        <AccordionTrigger className="-ml-2 w-fit flex-none justify-start gap-3 hover:no-underline [&>svg]:order-first [&>svg]:shrink-0">
          <div>
            <div className="font-semibold text-(--accordion-trigger-foreground)">
              Pinned services
            </div>
            <p className="mt-1 font-normal text-(--accordion-description-foreground)">
              Quick access to your most-used tools
            </p>
          </div>
        </AccordionTrigger>

        <AccordionContent className="text-foreground" contentClassName="px-0 pt-2 pb-6">
          <PinnedServicesGrid services={pinnedServices} />
        </AccordionContent>
      </AccordionItem>

      <AccordionItem value="all-services">
        <AccordionTrigger className="-ml-2 w-fit flex-none justify-start gap-3 hover:no-underline [&>svg]:order-first [&>svg]:shrink-0">
          <div>
            <div className="font-semibold text-(--accordion-trigger-foreground)">All services</div>
          </div>
        </AccordionTrigger>

        <AccordionContent className="text-foreground" contentClassName="px-0 pt-2 pb-6">
          <ServicesSection services={services} onTogglePin={onTogglePin} />
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  );
}
