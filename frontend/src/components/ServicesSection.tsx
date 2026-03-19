import { Search, List, LayoutGrid } from "lucide-react"
import { useEffect, useMemo, useRef, useState } from "react"
import type { Service } from "../api/listServices"
import { Input } from "../components/ui/input"
import { Button } from "./ui/button"
import { ToggleGroup, ToggleGroupItem } from "../components/ui/toggle-group"
import { ServicesTable } from "./ServiceTable"
import { ServicesGrid } from "./ServicesGrid"

type ServicesSectionProps = {
  services: Service[]
  onTogglePin: (serviceId: string, nextPinned: boolean) => void | Promise<void>
}

export function ServicesSection({
  services,
  onTogglePin,
}: ServicesSectionProps) {
  const [query, setQuery] = useState("")
  const [view, setView] = useState<"table" | "grid">("table")
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const isShortcut =
        (event.ctrlKey || event.metaKey) &&
        event.key.toLowerCase() === "k"

      if (!isShortcut) return

      event.preventDefault()
      inputRef.current?.focus()
      inputRef.current?.select()
    }

    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [])

  const filteredServices = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase()
    if (!normalizedQuery) return services

    return services.filter((service) => {
      const haystack = [
        service.name,
        service.description,
        ...service.category,
      ]
        .join(" ")
        .toLowerCase()

      return haystack.includes(normalizedQuery)
    })
  }, [services, query])

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 px-1 md:flex-row md:items-center md:justify-between">
        <div className="flex w-full max-w-[262px] overflow-visible">
          <Input
            ref={inputRef}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search"
            className="h-11 rounded-r-none rounded-l-[8px] border border-input px-3"
          />

          <Button
            type="button"
            className="h-11 w-[49px] rounded-l-none rounded-r-[8px] bg-[#9B3DCC] px-[13px] py-1 hover:bg-[#9B3DCC]/90"
            aria-label="Search"
            onClick={() => inputRef.current?.focus()}
          >
            <Search className="h-4 w-4" />
          </Button>
        </div>

        <ToggleGroup
          type="single"
          value={view}
          onValueChange={(value) => {
            if (value === "table" || value === "grid") {
              setView(value)
            }
          }}
          className="h-[46px] gap-1 rounded-[8px] border border-border bg-secondary p-1"
        >
          <ToggleGroupItem
            value="table"
            aria-label="Table view"
            className="h-9 w-9 !rounded-[6px] text-[#65748A] transition-none data-[state=on]:bg-accent data-[state=on]:text-foreground data-[state=on]:shadow-[0px_1px_2px_0px_#0000000D]"
          >
            <List className="h-4 w-4" />
          </ToggleGroupItem>

          <ToggleGroupItem
            value="grid"
            aria-label="Grid view"
            className="h-9 w-9 !rounded-[6px] text-[#65748A] transition-none data-[state=on]:bg-accent data-[state=on]:text-foreground data-[state=on]:shadow-[0px_1px_2px_0px_#0000000D]"
          >
            <LayoutGrid className="h-4 w-4" />
          </ToggleGroupItem>
        </ToggleGroup>
      </div>

      <div className="px-1">
        {view === "table" ? (
          <ServicesTable
            services={filteredServices}
            onTogglePin={onTogglePin}
          />
        ) : (
          <ServicesGrid
            services={filteredServices}
            onTogglePin={onTogglePin}
          />
        )}
      </div>
    </div>
  )
}
