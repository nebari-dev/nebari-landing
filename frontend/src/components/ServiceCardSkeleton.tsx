import { Card } from "./ui/card";
import { Skeleton } from "./ui/skeleton";

/** Loading placeholder with the shape of a service card. */
export function ServiceCardSkeleton() {
  return (
    <Card size="sm" aria-hidden="true" className="h-full gap-3 rounded-md p-4 shadow-none">
      <div className="flex items-center gap-3">
        <Skeleton shape="block" className="size-10" />
        <Skeleton className="w-27.5 max-w-full" />
      </div>
      <div className="flex flex-col gap-2 py-1">
        <Skeleton className="h-3" />
        <Skeleton className="h-3 w-50 max-w-full" />
      </div>
      <div className="flex items-center gap-2">
        <Skeleton shape="block" className="h-5 w-21" />
        <div className="flex-1" />
        <Skeleton className="h-3 w-12" />
      </div>
    </Card>
  );
}
