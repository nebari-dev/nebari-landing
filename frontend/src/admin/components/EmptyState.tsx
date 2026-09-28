import { CircleAlert, Inbox } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

type EmptyStateProps = {
  title: string;
  description?: ReactNode;
  action?: ReactNode;
  icon?: ReactNode;
  variant?: "default" | "error";
  className?: string;
};

/**
 * Shared zero-state block for detail panels and dialogs. Mirrors the
 * DataTable's built-in state block (same icon treatment, type scale and
 * spacing) so "nothing here" reads the same everywhere in the admin area.
 * An error variant promotes the region to an alert.
 */
export function EmptyState({
  title,
  description,
  action,
  icon,
  variant = "default",
  className,
}: EmptyStateProps) {
  return (
    <div
      role={variant === "error" ? "alert" : "status"}
      className={cn(
        "flex flex-col items-center justify-center gap-2 px-6 py-10 text-center",
        className,
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          "mb-1 [&_svg]:size-6",
          variant === "error" ? "text-destructive-foreground" : "text-muted-foreground",
        )}
      >
        {icon ?? (variant === "error" ? <CircleAlert /> : <Inbox />)}
      </span>
      <p className="text-sm font-medium text-foreground">{title}</p>
      {description ? (
        <div className="max-w-sm text-sm leading-5 text-muted-foreground">{description}</div>
      ) : null}
      {action ? <div className="mt-1">{action}</div> : null}
    </div>
  );
}
