import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import type { ServiceSort, SortKey } from "../lib/services";
import { cn } from "../lib/utils";
import { TableHead } from "./ui/table";

type SortableTableHeadProps = {
  label: string;
  sortKey: SortKey;
  sort: ServiceSort;
  onSort: (key: SortKey) => void;
  className?: string;
};

/** Column header that sorts the table by its column when clicked. */
export function SortableTableHead({
  label,
  sortKey,
  sort,
  onSort,
  className,
}: SortableTableHeadProps) {
  const active = sort.key === sortKey;
  const ariaSort = active ? (sort.direction === "asc" ? "ascending" : "descending") : "none";
  const Icon = !active ? ArrowUpDown : sort.direction === "asc" ? ArrowUp : ArrowDown;

  return (
    <TableHead
      scope="col"
      aria-sort={ariaSort}
      className={cn("[&>button]:px-3", className)}
      onClick={() => onSort(sortKey)}
    >
      <span className="min-w-0 flex-1 truncate">{label}</span>
      <span aria-hidden="true" className="flex size-6 shrink-0 items-center justify-center">
        <Icon
          strokeWidth={1.5}
          className={cn("size-3.5", active ? "text-foreground" : "text-muted-foreground")}
        />
      </span>
    </TableHead>
  );
}
