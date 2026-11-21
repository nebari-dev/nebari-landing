import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "../lib/utils";

type ServicesEmptyStateProps = {
  icon: LucideIcon;
  title: string;
  description: string;
  /** A follow-up action, such as clearing the search or retrying. */
  action?: ReactNode;
  /** `destructive` tints the icon for errors. */
  tone?: "default" | "destructive";
  /** `alert` for errors, `status` for results that change as the user types. */
  role?: "alert" | "status";
};

/** Dashed placeholder shown in place of the services when there is nothing to list. */
export function ServicesEmptyState({
  icon: Icon,
  title,
  description,
  action,
  tone = "default",
  role,
}: ServicesEmptyStateProps) {
  return (
    <div
      role={role}
      className="flex min-h-[max(20rem,calc(100dvh-18rem))] flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border px-6 py-12 text-center"
    >
      <div className="flex size-12 items-center justify-center rounded-full bg-muted">
        <Icon
          aria-hidden="true"
          strokeWidth={1.5}
          className={cn(
            "size-5.5",
            tone === "destructive" ? "text-destructive-foreground" : "text-muted-foreground-strong",
          )}
        />
      </div>
      <h2 className="text-base font-medium text-foreground">{title}</h2>
      <p className="max-w-95 text-sm text-muted-foreground-strong">{description}</p>
      {action && <div className="mt-1">{action}</div>}
    </div>
  );
}
