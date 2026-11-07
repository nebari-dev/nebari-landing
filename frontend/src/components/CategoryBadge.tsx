import { cn } from "../lib/utils";
import { Badge } from "./ui/badge";

type CategoryBadgeProps = {
  category: string;
  className?: string;
};

/** Category pill on service cards and table rows. */
export function CategoryBadge({ category, className }: CategoryBadgeProps) {
  return (
    <Badge
      variant="secondary"
      className={cn("bg-muted px-1.5 text-muted-foreground-strong", className)}
    >
      {category}
    </Badge>
  );
}
