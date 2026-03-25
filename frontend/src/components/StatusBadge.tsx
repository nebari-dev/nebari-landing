type StatusBadgeProps = {
  status: string
}

export function StatusBadge({ status }: StatusBadgeProps) {
  const normalizedStatus = status.toLowerCase()

  const isHealthy = normalizedStatus === "healthy"
  const isUnhealthy = normalizedStatus === "unhealthy"

  const displayStatus =
    status.charAt(0).toUpperCase() + status.slice(1).toLowerCase()

  return (
    <div
      className={[
        "inline-flex items-center gap-2 rounded-md px-2 py-1 text-xs font-medium",
        isHealthy && "bg-(--status-healthy-bg) text-(--status-healthy-fg)",
        isUnhealthy && "bg-(--status-unhealthy-bg) text-(--status-unhealthy-fg)",
        !isHealthy &&
          !isUnhealthy &&
          "bg-(--status-default-bg) text-(--status-default-fg)",
      ]
        .filter(Boolean)
        .join(" ")}
    >
      <span
        className={[
          "h-2 w-2 rounded-full",
          isHealthy && "bg-(--status-healthy-dot)",
          isUnhealthy && "bg-(--status-unhealthy-dot)",
          !isHealthy && !isUnhealthy && "bg-(--status-default-dot)",
        ]
          .filter(Boolean)
          .join(" ")}
      />
      {displayStatus}
    </div>
  )
}
