import type { ServiceSort, SortDirection, SortKey } from "../lib/services";
import { cn } from "../lib/utils";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./ui/select";

type SortValue = `${SortKey}-${SortDirection}`;

/** `label` is shown in the menu, `triggerLabel` on the closed trigger. */
const SORT_OPTIONS: { value: SortValue; label: string; triggerLabel: string }[] = [
  { value: "default-asc", label: "Default", triggerLabel: "Default" },
  { value: "name-asc", label: "Name (A–Z)", triggerLabel: "Name" },
  { value: "name-desc", label: "Name (Z–A)", triggerLabel: "Name (Z–A)" },
  { value: "category-asc", label: "Category (A–Z)", triggerLabel: "Category" },
  { value: "category-desc", label: "Category (Z–A)", triggerLabel: "Category (Z–A)" },
  { value: "status-asc", label: "Status (available first)", triggerLabel: "Status" },
  { value: "status-desc", label: "Status (unavailable first)", triggerLabel: "Status (reversed)" },
];

function toSortValue({ key, direction }: ServiceSort): SortValue {
  return `${key}-${direction}`;
}

function fromSortValue(value: SortValue): ServiceSort {
  const [key, direction] = value.split("-") as [SortKey, SortDirection];
  return { key, direction };
}

type SortSelectProps = {
  sort: ServiceSort;
  onChange: (sort: ServiceSort) => void;
  className?: string;
};

/** Sort order dropdown. */
export function SortSelect({ sort, onChange, className }: SortSelectProps) {
  return (
    <Select
      items={SORT_OPTIONS}
      value={toSortValue(sort)}
      onValueChange={(value) => {
        if (value) onChange(fromSortValue(value as SortValue));
      }}
    >
      <SelectTrigger aria-label="Sort services" className={cn("bg-card", className)}>
        <SelectValue>
          {(value: SortValue) =>
            `Sort: ${SORT_OPTIONS.find((option) => option.value === value)?.triggerLabel}`
          }
        </SelectValue>
      </SelectTrigger>
      <SelectContent align="start" sideOffset={4} className="w-auto min-w-(--anchor-width)">
        {SORT_OPTIONS.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
