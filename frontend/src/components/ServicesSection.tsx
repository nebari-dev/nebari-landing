import { LayoutGrid, List, Search } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import type { Service } from "../api/listServices";
import { Input } from "../components/ui/input";
import { Tabs, TabsList, TabsPanel, TabsTab } from "../components/ui/tabs";
import { ServicesGrid } from "./ServicesGrid";
import { ServicesTable } from "./ServiceTable";

type ServicesSectionProps = {
  services: Service[];
  onTogglePin: (serviceId: string, nextPinned: boolean) => void | Promise<void>;
};

export function ServicesSection({ services, onTogglePin }: ServicesSectionProps) {
  const [query, setQuery] = useState("");
  const [view, setView] = useState<"table" | "grid">("grid");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const isShortcut = (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k";

      if (!isShortcut) return;

      event.preventDefault();
      inputRef.current?.focus();
      inputRef.current?.select();
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const filteredServices = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    if (!normalizedQuery) return services;

    return services.filter((service) => {
      const haystack = [service.name, service.description, ...service.category]
        .join(" ")
        .toLowerCase();

      return haystack.includes(normalizedQuery);
    });
  }, [services, query]);

  return (
    <Tabs
      value={view}
      onValueChange={(value) => {
        if (value === "table" || value === "grid") {
          setView(value);
        }
      }}
    >
      <div className="grid items-center gap-4 px-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        <div className="relative min-w-0">
          <Search
            aria-hidden="true"
            className="pointer-events-none absolute top-1/2 left-3 z-10 size-4 -translate-y-1/2 text-muted-foreground"
          />
          <Input
            ref={inputRef}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search"
            aria-label="Search services"
            className="h-[34px] bg-card pl-9"
          />
        </div>

        <TabsList className="h-[34px] justify-self-end bg-muted p-1 sm:col-start-2 lg:col-start-3 xl:col-start-4">
          <TabsTab
            value="grid"
            className="h-auto gap-1 border-transparent bg-transparent px-1.5 py-0.5 text-muted-foreground-strong data-active:bg-card"
          >
            <LayoutGrid />
            <span>Grid View</span>
          </TabsTab>

          <TabsTab
            value="table"
            className="h-auto gap-1 border-transparent bg-transparent px-1.5 py-0.5 text-muted-foreground-strong data-active:bg-card"
          >
            <List />
            <span>List View</span>
          </TabsTab>
        </TabsList>
      </div>

      <TabsPanel value="table" tabIndex={-1} className="px-1">
        <ServicesTable services={filteredServices} onTogglePin={onTogglePin} />
      </TabsPanel>
      <TabsPanel value="grid" tabIndex={-1} className="px-1">
        <ServicesGrid services={filteredServices} onTogglePin={onTogglePin} />
      </TabsPanel>
    </Tabs>
  );
}
