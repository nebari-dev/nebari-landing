import type { Service } from "../api/listServices";
import { Table, TableBody, TableHead, TableHeader, TableRow } from "../components/ui/table";
import type { ServiceSort, SortKey } from "../lib/services";
import { ServiceTableRow } from "./ServiceTableRow";
import { ServiceTableRowSkeleton } from "./ServiceTableRowSkeleton";
import { SortableTableHead } from "./SortableTableHead";

type ServicesTableProps = {
  /** Already filtered and sorted. */
  services: Service[];
  sort: ServiceSort;
  onSortChange: (sort: ServiceSort) => void;
  onTogglePin?: (serviceId: string, nextPinned: boolean) => void;
  /** Shows skeleton rows under the headers while the services load. */
  loading?: boolean;
  /** Opens the details drawer for a service. */
  onShowDetails?: (serviceId: string) => void;
};

export function ServicesTable({
  services,
  sort,
  onSortChange,
  onTogglePin,
  loading = false,
  onShowDetails,
}: ServicesTableProps) {
  const orderedServices = [
    ...services.filter((service) => service.pinned),
    ...services.filter((service) => !service.pinned),
  ];

  const handleSort = (key: SortKey) => {
    onSortChange({
      key,
      direction: sort.key === key && sort.direction === "asc" ? "desc" : "asc",
    });
  };

  return (
    <Table
      aria-label="Services"
      className="min-w-140 table-fixed"
      scrollContainerClassName="focus-visible:ring-inset focus-visible:ring-offset-0"
    >
      <colgroup>
        <col className="w-13" />
        <col className="w-[42.5%]" />
        <col className="w-[18%]" />
        <col />
        <col className="w-31" />
      </colgroup>

      <TableHeader className="[&_th]:bg-header">
        <TableRow className="hover:bg-transparent">
          <TableHead scope="col" className="px-3">
            <span className="sr-only">Pinned</span>
          </TableHead>
          <SortableTableHead label="Service" sortKey="name" sort={sort} onSort={handleSort} />
          <SortableTableHead label="Category" sortKey="category" sort={sort} onSort={handleSort} />
          <SortableTableHead label="Status" sortKey="status" sort={sort} onSort={handleSort} />
          <TableHead scope="col" className="px-4">
            <span className="sr-only">Actions</span>
          </TableHead>
        </TableRow>
      </TableHeader>

      <TableBody>
        {loading
          ? Array.from({ length: 8 }, (_, index) => (
              <ServiceTableRowSkeleton key={index} index={index} />
            ))
          : orderedServices.map((service) => (
              <ServiceTableRow
                key={service.id}
                service={service}
                onTogglePin={onTogglePin}
                onShowDetails={onShowDetails}
              />
            ))}
      </TableBody>
    </Table>
  );
}
