import { Circle } from "lucide-react";
import { cn } from "../lib/utils";
import { Badge } from "./ui/badge";

type StatusBadgeProps = {
  status: string;
};

export function StatusBadge({ status }: StatusBadgeProps) {
  const normalizedStatus = status.toLowerCase();

  const isHealthy = normalizedStatus === "healthy";
  const isUnhealthy = normalizedStatus === "unhealthy";

  const displayStatus = status.charAt(0).toUpperCase() + status.slice(1).toLowerCase();
  return (
    <Badge
      variant="ghost"
      className={cn(
        isHealthy && "bg-(--status-healthy-bg) text-(--status-healthy-fg)",
        isUnhealthy && "bg-(--status-unhealthy-bg) text-(--status-unhealthy-fg)",
        !isHealthy && !isUnhealthy && "bg-(--status-default-bg) text-(--status-default-fg)",
      )}
    >
      <Circle
        className={cn(
          "size-2 fill-current",
          isHealthy && "text-(--status-healthy-dot)",
          isUnhealthy && "text-(--status-unhealthy-dot)",
          !isHealthy && !isUnhealthy && "text-(--status-default-dot)",
        )}
      />
      {displayStatus}
    </Badge>
  );
}
