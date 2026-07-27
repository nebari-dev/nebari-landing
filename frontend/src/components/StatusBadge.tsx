import { Circle } from "lucide-react";
import { Badge } from "./ui/badge";

type StatusBadgeProps = {
  status: string;
};

export function StatusBadge({ status }: StatusBadgeProps) {
  const normalizedStatus = status.toLowerCase();

  const isHealthy = normalizedStatus === "healthy";
  const isUnhealthy = normalizedStatus === "unhealthy";

  const displayStatus = status.charAt(0).toUpperCase() + status.slice(1).toLowerCase();
  const color = isHealthy
    ? "var(--status-healthy-fg)"
    : isUnhealthy
      ? "var(--status-unhealthy-fg)"
      : "var(--status-default-fg)";
  const backgroundColor = isHealthy
    ? "var(--status-healthy-bg)"
    : isUnhealthy
      ? "var(--status-unhealthy-bg)"
      : "var(--status-default-bg)";
  const dotColor = isHealthy
    ? "var(--status-healthy-dot)"
    : isUnhealthy
      ? "var(--status-unhealthy-dot)"
      : "var(--status-default-dot)";

  return (
    <Badge variant="ghost" style={{ backgroundColor, color }}>
      <Circle className="size-2 fill-current" style={{ color: dotColor }} />
      {displayStatus}
    </Badge>
  );
}
