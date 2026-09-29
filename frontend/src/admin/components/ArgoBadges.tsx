import { Badge } from "@/components/ui/badge";

/** ArgoCD sync status as a neutral-or-warning chip. */
export function SyncBadge({ status }: { status: string }) {
  if (status === "Synced") return <Badge variant="outline">Synced</Badge>;
  if (status === "OutOfSync")
    return <Badge className="bg-warning text-warning-foreground">Out of sync</Badge>;
  return <Badge variant="ghost">{status || "Unknown"}</Badge>;
}

/** ArgoCD health status, colored only when it is not Healthy. */
export function ArgoHealthBadge({ status }: { status: string }) {
  switch (status) {
    case "Healthy":
      return <Badge className="bg-success text-success-foreground">Healthy</Badge>;
    case "Degraded":
    case "Missing":
      return <Badge variant="destructive">{status}</Badge>;
    case "Progressing":
    case "Suspended":
      return <Badge className="bg-warning text-warning-foreground">{status}</Badge>;
    default:
      return <Badge variant="ghost">{status || "Unknown"}</Badge>;
  }
}
