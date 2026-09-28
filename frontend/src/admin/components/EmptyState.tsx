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
 * Shared zero-state block for lists, search results and detail panels. An
 * error variant promotes the region to an alert.
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
        "flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-border px-6 py-10 text-center",
        className,
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          "flex size-10 items-center justify-center rounded-full bg-muted [&_svg]:size-5",
          variant === "error" ? "text-destructive-foreground" : "text-muted-foreground-strong",
        )}
      >
        {icon ?? (variant === "error" ? <CircleAlert /> : <Inbox />)}
      </span>
      <p className="text-sm font-medium text-foreground">{title}</p>
      {description ? (
        <p className="max-w-prose text-sm text-muted-foreground">{description}</p>
      ) : null}
      {action ? <div className="pt-2">{action}</div> : null}
    </div>
  );
}
