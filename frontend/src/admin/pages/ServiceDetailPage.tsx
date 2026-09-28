import { BookOpen, ExternalLink, Plus, Settings } from "lucide-react";
import { useMemo, useState } from "react";
import { Link, useParams } from "react-router";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { CodeBlock, CodeBlockBody, CodeBlockHeader } from "@/components/ui/code-block";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import type { AdminGroup, AdminUser } from "../api/types";
import { EmptyState } from "../components/EmptyState";
import { GroupBadge, StatusBadge, VisibilityBadge } from "../components/EntityBadges";
import { PageHeader } from "../components/PageHeader";
import { PickerDialog } from "../components/PickerDialog";
import { UserPicker } from "../components/UserPicker";
import { useAdminWorld, useBulkUpdateUsers } from "../hooks/useAdminData";
import { effectiveUserCount, principalsForService } from "../lib/access";
import { displayName, pluralize } from "../lib/format";

export function ServiceDetailPage() {
  const { id = "" } = useParams();
  const { users, groups, services, isLoading } = useAdminWorld();
  const service = services.find((s) => s.id === id);
  const bulk = useBulkUpdateUsers();

  const [grantOpen, setGrantOpen] = useState(false);
  const [grantGroup, setGrantGroup] = useState<AdminGroup | null>(null);
  const [picked, setPicked] = useState<AdminUser[]>([]);

  const principals = useMemo(
    () => (service ? principalsForService(service, groups, users) : null),
    [service, groups, users],
  );
  const reach = service ? effectiveUserCount(service, groups, users) : 0;

  if (isLoading) {
    return (
      <output aria-label="Loading service" className="flex flex-col gap-3">
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-8 w-72" />
        <Skeleton className="h-64" shape="block" />
      </output>
    );
  }

  if (!service || !principals) {
    return (
      <EmptyState
        title="Service not found"
        action={
          <Button variant="outline" render={<Link to="/admin/services" />}>
            Back to services
          </Button>
        }
      />
    );
  }

  const gateGroups = service.requiredGroups
    .map((name) => groups.find((g) => g.name === name))
    .filter((g): g is AdminGroup => g !== undefined);

  // The gate is spec.auth on the NebariApp: auth disabled → public, enabled
  // with no groups → any signed-in user, enabled with groups → those groups.
  const yaml = [
    "spec:",
    "  auth:",
    `    enabled: ${service.visibility === "public" ? "false" : "true"}`,
    ...(service.requiredGroups.length > 0
      ? ["    groups:", ...service.requiredGroups.map((g) => `      - ${g}`)]
      : []),
  ].join("\n");

  return (
    <section aria-labelledby="service-title">
      <PageHeader
        crumbs={[{ label: "Services", to: "/admin/services" }, { label: service.displayName }]}
        title={
          <span id="service-title" className="flex items-center gap-2">
            {service.displayName}
            <VisibilityBadge visibility={service.visibility} />
          </span>
        }
        description={
          <span className="flex flex-wrap gap-x-3">
            <span className="font-mono text-xs">
              {service.namespace}/{service.name}
            </span>
            <span>{service.category}</span>
            <span>
              {reach === "everyone"
                ? "Reachable by everyone"
                : `${pluralize(reach, "user")} can reach it`}
            </span>
          </span>
        }
        actions={
          <>
            {gateGroups.length > 0 ? (
              <Button onClick={() => setGrantOpen(true)}>
                <Plus aria-hidden="true" />
                Grant access
              </Button>
            ) : null}
            {service.docsUrl ? (
              <Button
                variant="outline"
                render={<a href={service.docsUrl} target="_blank" rel="noopener noreferrer" />}
              >
                <BookOpen aria-hidden="true" />
                Docs
              </Button>
            ) : null}
            {service.settingsUrl ? (
              <Button
                variant="outline"
                render={<a href={service.settingsUrl} target="_blank" rel="noopener noreferrer" />}
              >
                <Settings aria-hidden="true" />
                Pack settings
                <ExternalLink aria-hidden="true" />
              </Button>
            ) : null}
          </>
        }
      />

      <div className="grid gap-6 lg:grid-cols-[2fr_1fr]">
        <Card>
          <CardHeader>
            <CardTitle>Who can reach this service</CardTitle>
            <CardDescription>
              {principals.kind === "everyone"
                ? "Public: no sign-in required."
                : principals.kind === "any-authenticated"
                  ? "Private with no required groups: any signed-in user."
                  : "Members of any listed group get through the gate."}
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-6">
            {principals.kind === "groups" ? (
              principals.entries.map((entry) => (
                <div key={entry.name}>
                  <div className="mb-2 flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2">
                      <GroupBadge id={entry.group?.id ?? null} name={entry.name} />
                      <span className="text-xs text-muted-foreground">
                        {entry.group
                          ? pluralize(entry.members.length, "member")
                          : "group does not exist in Keycloak"}
                      </span>
                    </div>
                    {entry.group ? (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          setGrantGroup(entry.group);
                          setGrantOpen(true);
                        }}
                      >
                        <Plus aria-hidden="true" />
                        Add to {entry.name}
                      </Button>
                    ) : null}
                  </div>
                  {entry.members.length === 0 ? (
                    <p className="text-sm text-muted-foreground">No members yet.</p>
                  ) : (
                    <ul className="grid gap-1 sm:grid-cols-2">
                      {entry.members.map((u) => (
                        <li key={u.id} className="flex items-center gap-2 text-sm">
                          <Link
                            to={`/admin/users/${encodeURIComponent(u.id)}`}
                            className="truncate hover:underline"
                          >
                            {displayName(u)}
                          </Link>
                          {!u.enabled ? <StatusBadge enabled={false} /> : null}
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              ))
            ) : (
              <EmptyState
                title={
                  principals.kind === "everyone"
                    ? "Open to everyone"
                    : "Open to all signed-in users"
                }
                description="To restrict it, enable spec.auth on the NebariApp and list the allowed groups under spec.auth.groups."
              />
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Gate definition</CardTitle>
            <CardDescription>
              Declared in the NebariApp CR and reconciled by GitOps. Edit it in the pack's
              repository, not here.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <CodeBlock code={yaml} className="w-full" showCopyButton={false}>
              <CodeBlockHeader>
                <span>nebariapp.yaml</span>
              </CodeBlockHeader>
              <CodeBlockBody />
            </CodeBlock>
            <a
              href={service.url}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
            >
              Open service
              <ExternalLink aria-hidden="true" className="size-3.5" />
            </a>
          </CardContent>
        </Card>
      </div>

      {gateGroups.length > 1 && grantGroup === null ? (
        <PickerDialog
          open={grantOpen}
          onOpenChange={setGrantOpen}
          title={`Grant access to ${service.displayName}`}
          description="Pick which of the service's groups to add people to."
          fieldLabel="Group"
          options={gateGroups.map((g) => ({
            value: g.id,
            label: g.name,
            description: g.description,
          }))}
          confirmLabel="Continue"
          onConfirm={(gid) => setGrantGroup(gateGroups.find((g) => g.id === gid) ?? null)}
        />
      ) : null}

      <Dialog
        open={grantOpen && (grantGroup !== null || gateGroups.length === 1)}
        onOpenChange={(o) => {
          if (!o) {
            setPicked([]);
            setGrantGroup(null);
          }
          setGrantOpen(o);
        }}
      >
        <DialogContent>
          {(() => {
            const target = grantGroup ?? gateGroups[0];
            if (!target) return null;
            const candidates = users.filter((u) => !u.groups.includes(target.id));
            return (
              <>
                <DialogHeader>
                  <DialogTitle>Grant access to {service.displayName}</DialogTitle>
                  <DialogDescription>
                    Adds the selected users to <strong>{target.name}</strong>. They also gain any
                    other service that group unlocks.
                  </DialogDescription>
                </DialogHeader>
                <UserPicker label="Users" users={candidates} value={picked} onChange={setPicked} />
                <DialogFooter>
                  <DialogClose render={<Button variant="outline" />}>Cancel</DialogClose>
                  <Button
                    disabled={picked.length === 0}
                    loading={bulk.isPending}
                    onClick={() =>
                      bulk.mutate(
                        {
                          userIds: picked.map((u) => u.id),
                          action: "addToGroup",
                          groupId: target.id,
                        },
                        {
                          onSuccess: () => {
                            setPicked([]);
                            setGrantGroup(null);
                            setGrantOpen(false);
                          },
                        },
                      )
                    }
                  >
                    Grant to {pluralize(picked.length, "user")}
                  </Button>
                </DialogFooter>
              </>
            );
          })()}
        </DialogContent>
      </Dialog>
    </section>
  );
}
