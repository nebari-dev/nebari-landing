import { cn } from "../lib/utils";
import { Badge } from "./ui/badge";

function categoryPillClassName(muted: boolean, className?: string) {
  return cn("bg-muted px-1.5 text-muted-foreground-strong", muted && "opacity-60", className);
}

type CategoryBadgeProps = {
  category: string;
  /** Fades the badge for an unavailable service. */
  muted?: boolean;
  className?: string;
};

/** Category pill on service cards and table rows. */
export function CategoryBadge({ category, muted = false, className }: CategoryBadgeProps) {
  return (
    <Badge variant="secondary" className={categoryPillClassName(muted, className)}>
      {category}
    </Badge>
  );
}

type CategoryBadgesProps = {
  categories: string[];
  /** Fades the badges for an unavailable service. */
  muted?: boolean;
  /** Extra classes for the category name pill. */
  badgeClassName?: string;
};

/** The first category, then a "+N" pill for the rest. */
export function CategoryBadges({ categories, muted = false, badgeClassName }: CategoryBadgesProps) {
  const [first, ...rest] = categories;
  if (!first) return null;

  return (
    <>
      <CategoryBadge category={first} muted={muted} className={badgeClassName} />
      {rest.length > 0 && (
        <Badge variant="secondary" title={rest.join(", ")} className={categoryPillClassName(muted)}>
          <span aria-hidden="true">+{rest.length}</span>
          <span className="sr-only">{`${rest.length} more categories: ${rest.join(", ")}`}</span>
        </Badge>
      )}
    </>
  );
}
