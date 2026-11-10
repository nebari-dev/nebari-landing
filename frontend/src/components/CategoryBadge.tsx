import { cn } from "../lib/utils";
import { Badge } from "./ui/badge";

const CATEGORY_PILL_CLASS = "bg-muted px-1.5 text-muted-foreground-strong";

type CategoryBadgeProps = {
  category: string;
  className?: string;
};

/** Category pill on service cards and table rows. */
export function CategoryBadge({ category, className }: CategoryBadgeProps) {
  return (
    <Badge variant="secondary" className={cn(CATEGORY_PILL_CLASS, className)}>
      {category}
    </Badge>
  );
}

type CategoryBadgesProps = {
  categories: string[];
  /** Extra classes for the category name pill. */
  badgeClassName?: string;
};

/** The first category, then a "+N" pill for the rest. */
export function CategoryBadges({ categories, badgeClassName }: CategoryBadgesProps) {
  const [first, ...rest] = categories;
  if (!first) return null;

  return (
    <>
      <CategoryBadge category={first} className={badgeClassName} />
      {rest.length > 0 && (
        <Badge variant="secondary" title={rest.join(", ")} className={CATEGORY_PILL_CLASS}>
          <span aria-hidden="true">+{rest.length}</span>
          <span className="sr-only">{`${rest.length} more categories: ${rest.join(", ")}`}</span>
        </Badge>
      )}
    </>
  );
}
