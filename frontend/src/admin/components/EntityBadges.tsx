import { Globe, Lock } from "lucide-react";
import { Link } from "react-router";
import { Badge } from "@/components/ui/badge";
import type { ServiceVisibility } from "../api/types";

/** Group chip linking to the group's detail page. */
export function GroupBadge({ id, name }: { id: string | null; name: string }) {
  if (!id) {
    return (
      <Badge
        variant="outline"
        className="border-dashed text-muted-foreground"
        title="Referenced by a service gate but not found in Keycloak"
      >
        {name}
      </Badge>
    );
  }
  return (
    <Badge variant="secondary" render={<Link to={`/admin/groups/${encodeURIComponent(id)}`} />}>
      {name}
    </Badge>
  );
}

/** Role chip linking to the role's detail page. */
export function RoleBadge({ name, inherited = false }: { name: string; inherited?: boolean }) {
  return (
    <Badge
      variant="outline"
      className={inherited ? "border-dashed text-muted-foreground" : undefined}
      render={<Link to={`/admin/roles/${encodeURIComponent(name)}`} />}
    >
      {name}
    </Badge>
  );
}

export function VisibilityBadge({ visibility }: { visibility: ServiceVisibility }) {
  return visibility === "public" ? (
    <Badge variant="outline">
      <Globe aria-hidden="true" />
      Public
    </Badge>
  ) : (
    <Badge variant="secondary">
      <Lock aria-hidden="true" />
      Private
    </Badge>
  );
}

export function StatusBadge({ enabled }: { enabled: boolean }) {
  return enabled ? (
    <Badge className="bg-success text-success-foreground">Active</Badge>
  ) : (
    <Badge variant="destructive">Disabled</Badge>
  );
}

/** Renders up to `max` chips followed by an overflow count. */
export function BadgeOverflow<T>({
  items,
  max = 3,
  render,
}: {
  items: T[];
  max?: number;
  render: (item: T) => React.ReactNode;
}) {
  const shown = items.slice(0, max);
  const rest = items.length - shown.length;
  return (
    <span className="flex flex-wrap items-center gap-1">
      {shown.map(render)}
      {rest > 0 ? (
        <Badge variant="ghost" className="text-muted-foreground">
          +{rest}
        </Badge>
      ) : null}
    </span>
  );
}
