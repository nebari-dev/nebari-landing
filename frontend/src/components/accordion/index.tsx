import {
  useCallback, useMemo, useState
} from "react";

import type {
  ReactNode
} from "react"

import {
  Accordion
} from "@trussworks/react-uswds";

import {
  ChevronDown
} from "lucide-react";

import "./index.scss";

import AppGrid from "../grid";
import ServicesPanel from "../servicesPanel";
import { SimpleCard } from "../card";

export type AppAccordionService = {
  id: string;
  image: string;
  name: string;
  status: string;
  description: string;
  category: string[];
  url: string;
  pinned: boolean;
};

export type AppAccordionProps = {
  pinnedServices: AppAccordionService[];
  services: AppAccordionService[];
};

export default function AppAccordion(props: AppAccordionProps): ReactNode {

  // Extract props
  const { pinnedServices, services } = props;

  const [expandedIds, setExpandedIds] = useState<string[]>(["pinned", "services"]);

  // Track locally toggled pin states before the server round-trip updates props.
  const [localPinOverrides, setLocalPinOverrides] = useState<Record<string, boolean>>({});

  // Derive services list from props + local overrides — no useEffect needed.
  const servicesState = useMemo(() => {
    const pinnedIds = new Set(pinnedServices.map((s) => s.id));
    return services.map((service) => ({
      ...service,
      pinned:
        service.id in localPinOverrides
          ? localPinOverrides[service.id]
          : pinnedIds.has(service.id) || !!service.pinned,
    }));
  }, [services, pinnedServices, localPinOverrides]);

  const toggleAccordion = useCallback((id: string) => {
    setExpandedIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    );
  }, []);

  /** Toggle pin state for a service optimistically until props refresh. */
  const pinService = useCallback((serviceId: string) => {
    const currentlyPinned =
      servicesState.find((s) => s.id === serviceId)?.pinned ?? false;
    setLocalPinOverrides((prev) => ({ ...prev, [serviceId]: !currentlyPinned }));
  }, [servicesState]);

  // Derived pinned list
  const pinnedList = useMemo(
    () => servicesState.filter((s) => s.pinned),
    [servicesState]
  );

  const accordionItems = useMemo(
    () => [
      {
        id: "pinned",
        headingLevel: "h2" as const,
        expanded: expandedIds.includes("pinned"),
        title: (
          <span className="app-accordion__trigger">
            <span
              className={`app-accordion__chevron ${expandedIds.includes("pinned") ? "is-open" : ""
                }`}
              aria-hidden="true"
            >
              <ChevronDown size={16} />
            </span>

            <span className="app-accordion__titleBlock">
              <span className="app-accordion__titleText">PINNED SERVICES</span>
              <span className="app-accordion__description">
                Quick access to your most-used tools
              </span>
            </span>
          </span>
        ),
        content: (
          <div className="app-accordion__content">
            {pinnedList.length > 0 ? (
              <AppGrid>
                {pinnedList.map((service) => (
                  <SimpleCard
                    key={service.id}
                    image={service.image}
                    name={service.name}
                    status={service.status}
                  />
                ))}
              </AppGrid>
            ) : (
              <p className="usa-prose">No pinned services yet.</p>
            )}
          </div>
        ),
        handleToggle: () => toggleAccordion("pinned"),
      },
      {
        id: "services",
        headingLevel: "h2" as const,
        expanded: expandedIds.includes("services"),
        title: (
          <span className="app-accordion__trigger">
            <span
              className={`app-accordion__chevron ${expandedIds.includes("services") ? "is-open" : ""
                }`}
              aria-hidden="true"
            >
              <ChevronDown size={16} />
            </span>

            <span className="app-accordion__titleText">ALL SERVICES</span>
          </span>
        ),
        content: (
          <div className="app-accordion__content">
            <ServicesPanel
              services={servicesState}
              onTogglePin={pinService}
            />
          </div>
        ),
        handleToggle: () => toggleAccordion("services"),
      },
    ],
    [expandedIds, pinnedList, servicesState, toggleAccordion, pinService]
  );

  return (
    <section className="app-accordion">
      <Accordion multiselectable items={accordionItems} />
    </section>
  );
}


