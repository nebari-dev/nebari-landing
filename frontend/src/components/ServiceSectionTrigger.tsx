import { ChevronDown } from "lucide-react";
import { AccordionTrigger } from "./ui/accordion";

type ServiceSectionTriggerProps = {
  title: string;
  count: number;
};

/** Accordion trigger with a section title and its service count. */
export function ServiceSectionTrigger({ title, count }: ServiceSectionTriggerProps) {
  return (
    <AccordionTrigger
      headingLevel={2}
      className="-ml-1 w-fit flex-none items-center px-1 py-0 hover:no-underline [&>[data-slot=accordion-trigger-icon]]:hidden"
    >
      <span className="flex items-center gap-2">
        <ChevronDown
          aria-hidden="true"
          strokeWidth={1.5}
          className="size-4 shrink-0 -rotate-90 text-muted-foreground-strong group-aria-expanded/accordion-trigger:rotate-0 motion-safe:transition-transform motion-safe:duration-(--duration-fast)"
        />
        <span className="text-base font-medium text-foreground">{title}</span>{" "}
        <span className="text-sm font-normal text-muted-foreground">{count}</span>
      </span>
    </AccordionTrigger>
  );
}
