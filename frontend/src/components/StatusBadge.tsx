import { cva, type VariantProps } from "class-variance-authority";
import type { ReactNode } from "react";
import type { HealthState, Service } from "../api/listServices";
import type { StatusView } from "../hooks/status-view";
import { cn } from "../lib/utils";
import { Badge } from "./ui/badge";

type ServiceHealth = Pick<Service, "status" | "health">;

/** The service's health state, read from the legacy status when the webapi sends no health. */
export function healthState({ status, health }: ServiceHealth): HealthState {
  if (health) return health;
  const normalized = status.toLowerCase();
  if (normalized === "healthy") return "healthy";
  if (normalized === "unhealthy") return "down";
  return "unknown";
}

/** True when a user sees the service as unavailable; admins see its health state instead. */
export function isUnavailable(service: ServiceHealth, view: StatusView) {
  return view === "user" && healthState(service) === "down";
}

const statusPillVariants = cva("px-1.5", {
  variants: {
    tone: {
      outline: "border-border bg-transparent text-foreground",
      success: "bg-success text-success-foreground",
      warning: "bg-warning text-warning-foreground",
      destructive: "bg-destructive text-destructive-foreground",
    },
  },
  defaultVariants: {
    tone: "outline",
  },
});

type StatusPillProps = VariantProps<typeof statusPillVariants> & {
  children: ReactNode;
  className?: string;
};

export function StatusPill({ tone = "outline", children, className }: StatusPillProps) {
  return (
    <Badge
      variant={tone === "outline" ? "outline" : "secondary"}
      className={cn(statusPillVariants({ tone }), className)}
    >
      {tone !== "outline" && (
        <span aria-hidden="true" className="size-2 shrink-0 rounded-full bg-current" />
      )}
      {children}
    </Badge>
  );
}

type StatusBadgeProps = ServiceHealth & {
  view: StatusView;
  /** The user's running sessions; ignored while a status pill takes its place. */
  sessionCount?: number;
};

/** The status pill for a service, or nothing when the service needs no callout. */
export function StatusBadge({ status, health, view, sessionCount = 0 }: StatusBadgeProps) {
  const state = healthState({ status, health });
  if (view === "admin") {
    if (state === "down") return <StatusPill tone="destructive">Down</StatusPill>;
    if (state === "degraded") return <StatusPill tone="warning">Degraded</StatusPill>;
  } else if (state === "down") {
    return <StatusPill>Unavailable</StatusPill>;
  }

  if (sessionCount > 0) {
    return (
      <StatusPill tone="success">
        {sessionCount} {sessionCount === 1 ? "session" : "sessions"} running
      </StatusPill>
    );
  }

  return null;
}
