import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "../components/ui/accordion"
import type { Service } from "../api/listServices"
import { cn } from "../lib/cn"
import { PinnedServicesGrid } from "./PinnedServicesGrid"
import { ServicesSection } from "./ServicesSection"

type ServicesAccordionProps = {
  pinnedServices: Service[]
  services: Service[]
  onTogglePin: (serviceId: string, nextPinned: boolean) => void | Promise<void>
}

export function ServicesAccordion({
  pinnedServices,
  services,
  onTogglePin,
}: ServicesAccordionProps) {
  return (
    <Accordion
      type="multiple"
      defaultValue={["pinned-services", "all-services"]}
      className="w-full"
    >
      <AccordionItem value="pinned-services" className="border-0">
        <AccordionTrigger
          className={cn(
            "relative z-10",
            "w-fit flex-none",
            "justify-start gap-3",
            "rounded-md py-4 pr-0",
            "transition-none hover:no-underline",
            "focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
            "[&>svg]:order-first [&>svg]:shrink-0"
          )}
        >
          <div className="text-left">
            <div className="text-sm font-semibold text-(--text-secondary)">
              Pinned services
            </div>
            <p className="mt-1 text-sm font-normal text-(--text-secondary)">
              Quick access to your most-used tools
            </p>
          </div>
        </AccordionTrigger>

        <AccordionContent className="pt-2 pb-6">
          <PinnedServicesGrid services={pinnedServices} />
        </AccordionContent>
      </AccordionItem>

      <AccordionItem value="all-services" className="border-0">
        <AccordionTrigger
          className={cn(
            "relative z-10",
            "w-fit flex-none",
            "justify-start gap-3",
            "rounded-md py-4 pr-0",
            "transition-none hover:no-underline",
            "focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
            "[&>svg]:order-first [&>svg]:shrink-0"
          )}
        >
          <div className="text-left">
            <div className="text-sm font-semibold text-(--text-secondary)">
              All services
            </div>
          </div>
        </AccordionTrigger>

        <AccordionContent className="pt-2 pb-6">
          <ServicesSection services={services} onTogglePin={onTogglePin} />
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  )
}
