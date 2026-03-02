import {
  useMemo, useState, useEffect
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


export default function AppAccordion(props: AppAccordion.Props): ReactNode {
  
  // Extract props
  const { pinnedServices, services } = props;

  // Accordion open/close state (keep your existing behavior)
  const [expandedIds, setExpandedIds] = useState<string[]>(["pinned", "services"]);

  const [view, setView] = useState<"grid" | "list">("list");
  const [search, setSearch] = useState("");

  const [servicesState, setServicesState] = useState<AppAccordion.Service[]>([]);

  // Sync local state from incoming props
  useEffect(() => {
    const pinnedIds = new Set(pinnedServices.map((s) => s.id));

    setServicesState(
      services.map((service) => ({
        ...service,
        pinned: pinnedIds.has(service.id) || !!service.pinned,
      }))
    );
  }, [services, pinnedServices]);

  const toggleAccordion = (id: string) => {
    setExpandedIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    );
  };

  /**
   * Toggle pin state for a service.
   * You asked for pinService(serviceId), so this toggles.
   * (If you want explicit set behavior later, add a second param.)
   */
  const pinService = (serviceId: string) => {
    setServicesState((prev) =>
      prev.map((service) =>
        service.id === serviceId
          ? { ...service, pinned: !service.pinned }
          : service
      )
    );
  };

  // Derived lists
  const pinnedList = useMemo(
    () => servicesState.filter((s) => s.pinned),
    [servicesState]
  );

  const filteredServices = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return servicesState;

    return servicesState.filter((s) => {
      const haystack = [
        s.name,
        s.status,
        s.description,
        ...(s.category ?? []),
      ]
        .join(" ")
        .toLowerCase();

      return haystack.includes(q);
    });
  }, [servicesState, search]);

  const accordionItems = useMemo(
    () => [
      {
        id: "pinned",
        headingLevel: "h2" as const,
        expanded: expandedIds.includes("pinned"),
        title: (
          <span className="app-accordion__trigger">
            <span
              className={`app-accordion__chevron ${
                expandedIds.includes("pinned") ? "is-open" : ""
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
              className={`app-accordion__chevron ${
                expandedIds.includes("services") ? "is-open" : ""
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
    [expandedIds, pinnedList, filteredServices, view, search]
  );

  return (
    <section className="app-accordion">
      <Accordion multiselectable items={accordionItems} />
    </section>
  );
}

export namespace AppAccordion {
  export type Service = {
    id: string;
    image: string;
    name: string;
    status: string;
    description: string;
    category: string[];
    pinned: boolean;
  };

  export type Props = {
    pinnedServices: Service[];
    services: Service[];
  };
}
