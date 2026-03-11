import { Tag } from "@trussworks/react-uswds";
import clsx from "clsx";

import "./index.scss";

export type StatusTagValue = "Healthy" | "Unhealthy" | "Unknown";

type StatusTagProps = {
  status: string;
};

const statusClassMap: Record<StatusTagValue, string> = {
  Healthy: "status-tag--healthy",
  Unhealthy: "status-tag--unhealthy",
  Unknown: "status-tag--unknown",
};

function normalizeStatus(status: string): StatusTagValue {
  switch (status.trim().toLowerCase()) {
    case "healthy":
      return "Healthy";
    case "unhealthy":
      return "Unhealthy";
    default:
      return "Unknown";
  }
}

export function StatusTag({ status }: StatusTagProps) {
  const normalizedStatus = normalizeStatus(status);

  return (
    <Tag className={clsx("status-tag", statusClassMap[normalizedStatus])}>
      {normalizedStatus}
    </Tag>
  );
}
