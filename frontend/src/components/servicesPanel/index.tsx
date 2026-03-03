import { useMemo, useState } from "react";
import type { ReactNode } from "react";

import AppGrid from "../grid";
import AppTable from "../table";
import AppSearchBar from "../search";
import AppViewToggle from "../buttongroup";
import { DetailedCard } from "../card";

import "./index.scss";

export default function ServicesPanel(props: ServicesPanelProps): ReactNode {
  const { services, onTogglePin } = props;

  const [view, setView] = useState<"grid" | "list">("list");
  const [search, setSearch] = useState("");

  const filteredServices = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return services;

    return services.filter((service) => {
      const haystack = [
        service.name,
        service.status,
        service.description,
        ...(service.category ?? []),
      ]
        .join(" ")
        .toLowerCase();

      return haystack.includes(q);
    });
  }, [services, search]);

  return (
    <div className="services-section">
      <div className="services-section__controls">
        <AppSearchBar
          className="services-section__search"
          onSubmit={(value) => setSearch(value)}
        />

        <AppViewToggle
          className="services-section__toggle"
          value={view}
          onChange={setView}
        />
      </div>

      {view === "list" ? (
        <AppTable
          caption="Services"
          rows={filteredServices.map((service) => ({
            id: service.id,
            title: service.name,
            description: service.description,
            status: service.status,
            categories: service.category,
            pinned: service.pinned,
            image: service.image,
          }))}
          onTogglePin={(rowId) => onTogglePin(rowId)}
        />
      ) : (
        <AppGrid>
          {filteredServices.map((service) => (
            <DetailedCard
              key={service.id}
              image={service.image}
              status={service.status}
              name={service.name}
              description={service.description}
              category={service.category}
              pinned={service.pinned}
              onTogglePin={() => onTogglePin(service.id)}
            />
          ))}
        </AppGrid>
      )}
    </div>
  );
}

export type ServicesPanelService = {
  id: string;
  image: string;
  name: string;
  status: string;
  description: string;
  category: string[];
  pinned: boolean;
};

export type ServicesPanelProps = {
  services: ServicesPanelService[];
  onTogglePin: (serviceId: string) => void;
};
