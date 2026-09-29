import type { ReactNode } from "react";
import { Link } from "react-router";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

type StatTileProps = {
  /** Sentence case, no trailing colon. */
  label: string;
  value: number | string | null | undefined;
  /** Short supporting line under the value, e.g. "3 disabled". */
  detail?: ReactNode;
  /** Routes the whole tile. */
  to?: string;
  /** Marks the tile as needing attention (non-zero problem count). */
  tone?: "default" | "warning" | "danger";
  loading?: boolean;
};

const COMPACT = new Intl.NumberFormat(undefined, { notation: "compact", maximumFractionDigits: 1 });

/** KPI tile: label, headline figure, supporting detail. */
export function StatTile({ label, value, detail, to, tone = "default", loading }: StatTileProps) {
  const display =
    value === null || value === undefined
      ? "—"
      : typeof value === "number"
        ? COMPACT.format(value)
        : value;
  const body = (
    <CardContent className="flex flex-col gap-1 p-4">
      <p className="text-sm text-muted-foreground">{label}</p>
      {loading ? (
        <Skeleton className="h-8 w-16" />
      ) : (
        <p
          className={cn(
            "text-3xl font-semibold leading-none text-foreground",
            tone === "warning" && "text-warning-foreground",
            tone === "danger" && "text-destructive-foreground",
          )}
        >
          {display}
        </p>
      )}
      {detail ? <div className="text-xs text-muted-foreground">{detail}</div> : null}
    </CardContent>
  );
  if (to) {
    return (
      <Card
        size="sm"
        className="p-0 outline-none transition-colors hover:border-border-strong focus-within:ring-2 focus-within:ring-ring"
      >
        <Link
          to={to}
          aria-label={`${label}: ${display}`}
          className="block rounded-[inherit] outline-none"
        >
          {body}
        </Link>
      </Card>
    );
  }
  return (
    <Card size="sm" className="p-0">
      {body}
    </Card>
  );
}
