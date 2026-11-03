import { cn } from "../lib/utils";
import { Badge } from "./ui/badge";

type CategoryBadgeProps = {
  category: string;
  /** Fades the badge for an unavailable service. */
  muted?: boolean;
  className?: string;
};

/** Category pill on service cards and table rows. */
export function CategoryBadge({ category, muted = false, className }: CategoryBadgeProps) {
  return (
    <Badge
      variant="secondary"
      className={cn(
        "bg-muted px-1.5 text-muted-foreground-strong",
        muted && "opacity-60",
        className,
      )}
    >
      {category}
    </Badge>
  );
}
