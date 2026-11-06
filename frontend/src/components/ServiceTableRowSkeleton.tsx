import { Skeleton } from "./ui/skeleton";
import { TableCell, TableRow } from "./ui/table";

const NAME_WIDTHS = ["w-25", "w-31", "w-37"];
const DESCRIPTION_WIDTHS = ["w-60", "w-67.5", "w-75", "w-82.5"];
const CATEGORY_WIDTHS = ["w-19", "w-22.5", "w-26"];

type ServiceTableRowSkeletonProps = {
  /** Picks the placeholder widths. */
  index: number;
};

/** Loading placeholder with the shape of a service table row. */
export function ServiceTableRowSkeleton({ index }: ServiceTableRowSkeletonProps) {
  return (
    <TableRow aria-hidden="true" className="hover:bg-transparent">
      <TableCell className="h-14 px-3 py-0" />

      <TableCell className="h-14 p-2">
        <div className="flex min-w-0 items-center gap-3">
          <Skeleton shape="circle" />
          <div className="flex min-w-0 flex-1 flex-col gap-2">
            <Skeleton className={`max-w-full ${NAME_WIDTHS[index % NAME_WIDTHS.length]}`} />
            <Skeleton
              className={`h-3 max-w-full ${DESCRIPTION_WIDTHS[index % DESCRIPTION_WIDTHS.length]}`}
            />
          </div>
        </div>
      </TableCell>

      <TableCell className="h-14 px-3">
        <Skeleton
          shape="block"
          className={`h-5 max-w-full rounded-full ${CATEGORY_WIDTHS[index % CATEGORY_WIDTHS.length]}`}
        />
      </TableCell>

      <TableCell className="h-14 px-3" />
    </TableRow>
  );
}
