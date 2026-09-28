import { ExternalLink, Plus, X } from "lucide-react";
import { useMemo, useState } from "react";
import { Link, useParams } from "react-router";
import { getInitials } from "@/auth/user";
import { Avatar, AvatarFallback } from "@/components/Avatar";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsList, TabsPanel, TabsTab } from "@/components/ui/tabs";
import { EmptyState } from "../components/EmptyState";
import { GroupBadge, RoleBadge, StatusBadge, VisibilityBadge } from "../components/EntityBadges";
import { PageHeader } from "../components/PageHeader";
import { PickerDialog } from "../components/PickerDialog";
import {
  useAddUserToGroup,
  useAdminWorld,
  useAssignUserRole,
  useRemoveUserFromGroup,
  useSetUserEnabled,
  useUnassignUserRole,
} from "../hooks/useAdminData";
import { effectiveRoles, servicesForUser } from "../lib/access";
import { displayName, formatDateTime, pluralize } from "../lib/format";
import { keycloakConsoleUrl } from "../lib/keycloakConsole";

export function UserDetailPage() {
  const { id = "" } = useParams();
  const { users, groups, roles, services, isLoading } = useAdminWorld();
  const user = users.find((u) => u.id === id);

  const setEnabled = useSetUserEnabled();
  const addToGroup = useAddUserToGroup();
  const removeFromGroup = useRemoveUserFromGroup();
  const assignRole = useAssignUserRole();
  const unassignRole = useUnassignUserRole();

  const [dialog, setDialog] = useState<"group" | "role" | null>(null);

  const access = useMemo(
    () => (user ? servicesForUser(user, groups, services) : []),
    [user, groups, services],
  );
  const roleInfo = useMemo(
    () => (user ? effectiveRoles(user, groups) : { direct: [], inherited: [] }),
    [user, groups],
  );

  if (isLoading) {
    return (
      <output aria-label="Loading user" className="flex flex-col gap-3">
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-8 w-72" />
        <Skeleton className="h-64" shape="block" />
      </output>
    );
  }

  if (!user) {
    return (
      <EmptyState
        title="User not found"
        description="It may have been deleted in Keycloak."
        action={
          <Button variant="outline" render={<Link to="/admin/users" />}>
            Back to users
          </Button>
        }
      />
    );
  }

  const memberGroups = groups.filter((g) => user.groups.includes(g.id));
  const availableGroups = groups.filter((g) => !user.groups.includes(g.id));
  const availableRoles = roles.filter((r) => !r.builtIn && !user.roles.includes(r.name));

  return (
    <section aria-labelledby="user-title">
      <PageHeader
        crumbs={[{ label: "Users", to: "/admin/users" }, { label: displayName(user) }]}
        leading={
          <Avatar className="size-12">
            <AvatarFallback className="bg-primary text-base font-semibold text-primary-foreground">
              {getInitials(displayName(user))}
            </AvatarFallback>
          </Avatar>
        }
        title={
          <span id="user-title" className="flex items-center gap-2">
            {displayName(user)}
            <StatusBadge enabled={user.enabled} />
          </span>
        }
        description={
          <span className="flex flex-wrap gap-x-3">
            <span className="font-mono text-xs">{user.username}</span>
            <span>{user.email}</span>
            <span>Last sign-in {formatDateTime(user.lastSignInAt)}</span>
          </span>
        }
        actions={
          <>
            <Label className="flex items-center gap-2 text-sm">
              <Switch
                checked={user.enabled}
                disabled={setEnabled.isPending}
                onCheckedChange={(checked) => setEnabled.mutate({ id: user.id, enabled: checked })}
              />
              {user.enabled ? "Account enabled" : "Account disabled"}
            </Label>
            <Button
              variant="outline"
              render={
                <a
                  href={keycloakConsoleUrl(`/users/${encodeURIComponent(user.id)}/settings`)}
                  target="_blank"
                  rel="noopener noreferrer"
                />
              }
            >
              Open in Keycloak
              <ExternalLink aria-hidden="true" />
            </Button>
          </>
        }
      />

      <Tabs defaultValue="access">
        <TabsList aria-label="User details">
          <TabsTab value="access">Access ({access.length})</TabsTab>
          <TabsTab value="groups">Groups ({memberGroups.length})</TabsTab>
          <TabsTab value="roles">
            Roles ({roleInfo.direct.length + roleInfo.inherited.length})
          </TabsTab>
        </TabsList>

        <TabsPanel value="access" className="pt-6">
          <div className="flex flex-col gap-4">
            <PanelHeader
              title="Services this user can reach"
              description="Derived from group membership and each service's gate. To change it, change the user's groups."
            />
            <div>
              {access.length === 0 ? (
                <EmptyState
                  title="No services"
                  description="This user is not in any group that unlocks a service."
                  action={
                    <Button onClick={() => setDialog("group")}>
                      <Plus aria-hidden="true" />
                      Add to group
                    </Button>
                  }
                />
              ) : (
                <Table aria-label="Accessible services">
                  <TableHeader>
                    <TableRow>
                      <TableHead>Service</TableHead>
                      <TableHead>Visibility</TableHead>
                      <TableHead>Granted by</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {access.map(({ service, reason }) => (
                      <TableRow key={service.id}>
                        <TableCell>
                          <Link
                            to={`/admin/services/${encodeURIComponent(service.id)}`}
                            className="font-medium hover:underline"
                          >
                            {service.displayName}
                          </Link>
                          <span className="block text-xs text-muted-foreground">
                            {service.category}
                          </span>
                        </TableCell>
                        <TableCell>
                          <VisibilityBadge visibility={service.visibility} />
                        </TableCell>
                        <TableCell>
                          {reason.kind === "public" ? (
                            <span className="text-muted-foreground">Everyone</span>
                          ) : reason.kind === "any-authenticated" ? (
                            <span className="text-muted-foreground">Any signed-in user</span>
                          ) : (
                            <span className="flex flex-wrap gap-1">
                              {reason.groups.map((name) => (
                                <GroupBadge
                                  key={name}
                                  id={groups.find((g) => g.name === name)?.id ?? null}
                                  name={name}
                                />
                              ))}
                            </span>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </div>
          </div>
        </TabsPanel>

        <TabsPanel value="groups" className="pt-6">
          <div className="flex flex-col gap-4">
            <PanelHeader
              title="Group membership"
              description="Groups are the unit of access. Membership changes apply on the user's next token refresh."
              action={
                <Button size="sm" onClick={() => setDialog("group")}>
                  <Plus aria-hidden="true" />
                  Add to group
                </Button>
              }
            />
            <div>
              {memberGroups.length === 0 ? (
                <EmptyState title="Not in any group" />
              ) : (
                <ul className="divide-y divide-border">
                  {memberGroups.map((g) => (
                    <li key={g.id} className="flex items-center justify-between gap-3 py-2">
                      <div className="min-w-0">
                        <GroupBadge id={g.id} name={g.name} />
                        <p className="mt-1 text-xs text-muted-foreground">
                          {g.description || "No description"}
                          {g.roles.length > 0 ? ` · grants ${g.roles.join(", ")}` : ""}
                        </p>
                      </div>
                      <Button
                        size="sm"
                        variant="ghost"
                        aria-label={`Remove from ${g.name}`}
                        loading={removeFromGroup.isPending}
                        onClick={() => removeFromGroup.mutate({ id: user.id, groupId: g.id })}
                      >
                        <X aria-hidden="true" />
                        Remove
                      </Button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        </TabsPanel>

        <TabsPanel value="roles" className="pt-6">
          <div className="flex flex-col gap-4">
            <PanelHeader
              title="Realm roles"
              description="Direct roles are mapped to the user; inherited roles come from groups and are removed by leaving the group."
              action={
                <Button size="sm" onClick={() => setDialog("role")}>
                  <Plus aria-hidden="true" />
                  Assign role
                </Button>
              }
            />
            <div className="flex flex-col gap-4">
              <div>
                <h4 className="mb-2 text-sm font-medium">
                  Direct ({pluralize(roleInfo.direct.length, "role")})
                </h4>
                {roleInfo.direct.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No direct roles.</p>
                ) : (
                  <ul className="flex flex-wrap gap-2">
                    {roleInfo.direct.map((r) => (
                      <li key={r} className="flex items-center gap-1">
                        <RoleBadge name={r} />
                        <Button
                          size="icon"
                          variant="ghost"
                          className="size-6"
                          aria-label={`Remove role ${r}`}
                          onClick={() => unassignRole.mutate({ id: user.id, role: r })}
                        >
                          <X aria-hidden="true" />
                        </Button>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
              <div>
                <h4 className="mb-2 text-sm font-medium">Inherited from groups</h4>
                {roleInfo.inherited.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No inherited roles.</p>
                ) : (
                  <ul className="flex flex-wrap gap-2">
                    {roleInfo.inherited.map(({ role, viaGroup }) => (
                      <li key={`${role}-${viaGroup.id}`} className="flex items-center gap-1">
                        <RoleBadge name={role} inherited />
                        <span className="text-xs text-muted-foreground">via {viaGroup.name}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>
          </div>
        </TabsPanel>
      </Tabs>

      <PickerDialog
        open={dialog === "group"}
        onOpenChange={(o) => !o && setDialog(null)}
        title={`Add ${displayName(user)} to a group`}
        fieldLabel="Group"
        options={availableGroups.map((g) => ({
          value: g.id,
          label: g.name,
          description: g.description,
        }))}
        emptyTitle="Already in every group"
        confirmLabel="Add to group"
        pending={addToGroup.isPending}
        onConfirm={(groupId) =>
          addToGroup.mutate({ id: user.id, groupId }, { onSuccess: () => setDialog(null) })
        }
      />
      <PickerDialog
        open={dialog === "role"}
        onOpenChange={(o) => !o && setDialog(null)}
        title={`Assign a role to ${displayName(user)}`}
        fieldLabel="Role"
        options={availableRoles.map((r) => ({
          value: r.name,
          label: r.name,
          description: r.description,
        }))}
        emptyTitle="Every role is already assigned"
        confirmLabel="Assign role"
        pending={assignRole.isPending}
        onConfirm={(role) =>
          assignRole.mutate({ id: user.id, role }, { onSuccess: () => setDialog(null) })
        }
      />
    </section>
  );
}

/** Title row for a tab panel: heading, supporting copy, optional action. */
function PanelHeader({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <h3 className="text-base font-semibold text-foreground">{title}</h3>
        {description ? <p className="text-sm text-muted-foreground">{description}</p> : null}
      </div>
      {action ? <div className="shrink-0">{action}</div> : null}
    </div>
  );
}
