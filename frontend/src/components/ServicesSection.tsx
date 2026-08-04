import { LayoutGrid, List, Search } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import type { Service } from "../api/listServices";
import { Input } from "../components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/tabs";
import { ServicesGrid } from "./ServicesGrid";
import { ServicesTable } from "./ServiceTable";

type ServicesSectionProps = {
  services: Service[];
  onTogglePin: (serviceId: string, nextPinned: boolean) => void | Promise<void>;
};

const viewToggleTriggerClassName =
  "h-auto gap-1 rounded-[6px] px-1.5 py-0.5 text-control-muted-foreground transition-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none data-active:border-border-strong data-active:bg-card data-active:text-foreground data-active:shadow-[0_1px_3px_0_rgba(0,0,0,0.10)] dark:data-active:bg-card";

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
      activationMode="manual"
      value={view}
      onValueChange={(value) => {
        if (value === "table" || value === "grid") {
          setView(value);
        }
      }}
      className="gap-4"
    >
      <div className="flex items-center justify-between gap-3 px-1">
        <div className="relative w-[33.333vw] max-w-[calc(100%_-_5.5rem)] min-w-0">
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
            className="h-[34px] bg-table-cell-background pl-9"
          />
        </div>

        <TabsList className="!h-[34px] shrink-0 gap-1 rounded-[8px] bg-muted p-1">
          <TabsTrigger value="grid" className={viewToggleTriggerClassName}>
            <LayoutGrid className="h-4 w-4" />
            <span>Grid View</span>
          </TabsTrigger>

          <TabsTrigger value="table" className={viewToggleTriggerClassName}>
            <List className="h-4 w-4" />
            <span>List View</span>
          </TabsTrigger>
        </TabsList>
      </div>

      <TabsContent value="table" tabIndex={-1} className="px-1">
        <ServicesTable services={filteredServices} onTogglePin={onTogglePin} />
      </TabsContent>
      <TabsContent value="grid" tabIndex={-1} className="px-1">
        <ServicesGrid services={filteredServices} onTogglePin={onTogglePin} />
      </TabsContent>
    </Tabs>
  );
}
