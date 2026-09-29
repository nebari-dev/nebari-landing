import { Badge } from "@/components/ui/badge";
import type { Pack } from "../api/types";

/** Installed-vs-published chart version; colored only when an update exists. */
export function VersionBadge({ pack }: { pack: Pick<Pack, "versionStatus" | "latestVersion"> }) {
  switch (pack.versionStatus) {
    case "behind":
      return (
        <Badge className="bg-warning text-warning-foreground">Update {pack.latestVersion}</Badge>
      );
    case "current":
      return <Badge variant="ghost">Latest</Badge>;
    case "ahead":
      return <Badge variant="ghost">Ahead of {pack.latestVersion}</Badge>;
    default:
      return null;
  }
}
