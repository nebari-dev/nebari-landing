import { Plus, Trash2, X } from "lucide-react";
import { useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { DataTable, type DataTableColumnDef } from "@/components/ui/data-table";
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
import { Tabs, TabsList, TabsPanel, TabsTab } from "@/components/ui/tabs";
import { GroupInUseError } from "../api/groups";
import type { AdminUser } from "../api/types";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { EmptyState } from "../components/EmptyState";
import { RoleBadge, StatusBadge, VisibilityBadge } from "../components/EntityBadges";
import { PageHeader } from "../components/PageHeader";
import { PickerDialog } from "../components/PickerDialog";
import { UserPicker } from "../components/UserPicker";
import {
  useAddGroupRole,
  useAdminWorld,
  useBulkUpdateUsers,
  useDeleteGroup,
  useRemoveGroupMember,
  useRemoveGroupRole,
} from "../hooks/useAdminData";
import { servicesUnlockedByGroup } from "../lib/access";
import { displayName, formatDate, pluralize } from "../lib/format";

type MemberRow = AdminUser & Record<string, unknown> & { search: string };

export function GroupDetailPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const { users, groups, roles, services, isLoading } = useAdminWorld();
  const group = groups.find((g) => g.id === id);

  const bulk = useBulkUpdateUsers();
  const removeMember = useRemoveGroupMember();
  const addRole = useAddGroupRole();
  const removeRole = useRemoveGroupRole();
  const deleteGroup = useDeleteGroup();

  const [addOpen, setAddOpen] = useState(false);
  const [roleOpen, setRoleOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteBlockedBy, setDeleteBlockedBy] = useState<string[] | null>(null);
  const [picked, setPicked] = useState<AdminUser[]>([]);

  const members = useMemo(
    () =>
      group
        ? users
            .filter((u) => u.groups.includes(group.id))
            .map<MemberRow>((u) => ({ ...u, search: `${displayName(u)} ${u.username} ${u.email}` }))
        : [],
    [users, group],
  );
  const nonMembers = useMemo(
    () => (group ? users.filter((u) => !u.groups.includes(group.id)) : []),
    [users, group],
  );
  const unlocked = useMemo(
    () => (group ? servicesUnlockedByGroup(group, services) : []),
    [group, services],
  );

  const columns = useMemo<DataTableColumnDef<MemberRow>[]>(
    () => [
      {
        id: "search",
        accessorKey: "search",
        header: "Member",
        cell: ({ row }) => (
          <Link
            to={`/admin/users/${encodeURIComponent(row.original.id)}`}
            className="flex min-w-0 flex-col rounded-sm outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
          >
            <span className="font-medium text-foreground">{displayName(row.original)}</span>
            <span className="truncate text-xs text-muted-foreground">{row.original.email}</span>
          </Link>
        ),
      },
      {
        id: "enabled",
        accessorKey: "enabled",
        header: "Status",
        cell: ({ row }) => <StatusBadge enabled={row.original.enabled} />,
      },
      {
        id: "lastSignInAt",
        accessorKey: "lastSignInAt",
        header: "Last sign-in",
        cell: ({ row }) => (
          <span className="text-muted-foreground">{formatDate(row.original.lastSignInAt)}</span>
        ),
      },
      {
        id: "actions",
        header: "",
        enableSorting: false,
        cell: ({ row }) =>
          group ? (
            <Button
              size="sm"
              variant="ghost"
              aria-label={`Remove ${displayName(row.original)} from ${group.name}`}
              onClick={() => removeMember.mutate({ id: group.id, userId: row.original.id })}
            >
              <X aria-hidden="true" />
              Remove
            </Button>
          ) : null,
      },
    ],
    [group, removeMember],
  );

  if (isLoading) {
    return (
      <output aria-label="Loading group" className="flex flex-col gap-3">
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-8 w-72" />
        <Skeleton className="h-64" shape="block" />
      </output>
    );
  }

  if (!group) {
    return (
      <EmptyState
        title="Group not found"
        action={
          <Button variant="outline" render={<Link to="/admin/groups" />}>
            Back to groups
          </Button>
        }
      />
    );
  }

  return (
    <section aria-labelledby="group-title">
      <PageHeader
        crumbs={[{ label: "Groups", to: "/admin/groups" }, { label: group.name }]}
        title={<span id="group-title">{group.name}</span>}
        description={
          <span className="flex flex-wrap gap-x-3">
            <span className="font-mono text-xs">{group.path}</span>
            <span>{group.description || "No description"}</span>
          </span>
        }
        actions={
          <>
            <Button onClick={() => setAddOpen(true)}>
              <Plus aria-hidden="true" />
              Add members
            </Button>
            <Button variant="outline" onClick={() => setDeleteOpen(true)}>
              <Trash2 aria-hidden="true" />
              Delete group
            </Button>
          </>
        }
      />

      <Tabs defaultValue="members">
        <TabsList aria-label="Group details">
          <TabsTab value="members">Members ({members.length})</TabsTab>
          <TabsTab value="services">Unlocks ({unlocked.length})</TabsTab>
          <TabsTab value="roles">Roles ({group.roles.length})</TabsTab>
        </TabsList>

        <TabsPanel value="members" className="pt-4">
          <DataTable
            ariaLabel={`Members of ${group.name}`}
            columns={columns}
            data={members}
            getRowId={(r) => r.id}
            getRowLabel={(r) => displayName(r)}
            filterColumnId="search"
            filterLabel="Search members"
            filterPlaceholder="Search members…"
            initialPageSize={25}
            showPagination={members.length > 25}
            selectable
            emptyTitle="No members yet"
            emptyDescription="Add users to grant them everything this group unlocks."
            emptyAction={
              <Button onClick={() => setAddOpen(true)}>
                <Plus aria-hidden="true" />
                Add members
              </Button>
            }
            filteredEmptyTitle="No members match"
            filteredEmptyDescription="Try a different search."
            selectionActions={(selected) => (
              <Button
                size="sm"
                variant="destructive"
                loading={bulk.isPending}
                onClick={() =>
                  bulk.mutate({
                    userIds: selected.map((u) => u.id),
                    action: "removeFromGroup",
                    groupId: group.id,
                  })
                }
              >
                Remove {pluralize(selected.length, "member")}
              </Button>
            )}
          />
        </TabsPanel>

        <TabsPanel value="services" className="pt-4">
          <Card>
            <CardHeader>
              <CardTitle>Services this group unlocks</CardTitle>
              <CardDescription>
                Each service's gate is declared in its NebariApp and reconciled by GitOps; it cannot
                be edited here.
              </CardDescription>
            </CardHeader>
            <CardContent>
              {unlocked.length === 0 ? (
                <EmptyState
                  title="No service references this group"
                  description={`Add "${group.name}" to a NebariApp's landingPage.requiredGroups to gate that service on it.`}
                />
              ) : (
                <ul className="divide-y divide-border">
                  {unlocked.map((s) => (
                    <li key={s.id} className="flex items-center justify-between gap-3 py-2">
                      <div className="min-w-0">
                        <Link
                          to={`/admin/services/${encodeURIComponent(s.id)}`}
                          className="font-medium hover:underline"
                        >
                          {s.displayName}
                        </Link>
                        <p className="text-xs text-muted-foreground">
                          {s.category} · also open to{" "}
                          {s.requiredGroups.filter((g) => g !== group.name).join(", ") ||
                            "no other group"}
                        </p>
                      </div>
                      <VisibilityBadge visibility={s.visibility} />
                    </li>
                  ))}
                </ul>
              )}
            </CardContent>
          </Card>
        </TabsPanel>

        <TabsPanel value="roles" className="pt-4">
          <Card>
            <CardHeader>
              <CardTitle>Roles granted to members</CardTitle>
              <CardDescription>
                Realm roles mapped to the group. Every member inherits them.
              </CardDescription>
              <div className="pt-2">
                <Button size="sm" onClick={() => setRoleOpen(true)}>
                  <Plus aria-hidden="true" />
                  Add role
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              {group.roles.length === 0 ? (
                <EmptyState title="No roles mapped" />
              ) : (
                <ul className="flex flex-wrap gap-2">
                  {group.roles.map((r) => (
                    <li key={r} className="flex items-center gap-1">
                      <RoleBadge name={r} />
                      <Button
                        size="icon"
                        variant="ghost"
                        className="size-6"
                        aria-label={`Remove role ${r}`}
                        onClick={() => removeRole.mutate({ id: group.id, role: r })}
                      >
                        <X aria-hidden="true" />
                      </Button>
                    </li>
                  ))}
                </ul>
              )}
            </CardContent>
          </Card>
        </TabsPanel>
      </Tabs>

      <Dialog
        open={addOpen}
        onOpenChange={(o) => {
          if (!o) setPicked([]);
          setAddOpen(o);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add members to {group.name}</DialogTitle>
            <DialogDescription>
              They gain{" "}
              {unlocked.length > 0
                ? unlocked.map((s) => s.displayName).join(", ")
                : "no services yet"}
              {group.roles.length > 0 ? ` and the ${group.roles.join(", ")} role(s)` : ""}.
            </DialogDescription>
          </DialogHeader>
          <UserPicker label="Users" users={nonMembers} value={picked} onChange={setPicked} />
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>Cancel</DialogClose>
            <Button
              disabled={picked.length === 0}
              loading={bulk.isPending}
              onClick={() =>
                bulk.mutate(
                  { userIds: picked.map((u) => u.id), action: "addToGroup", groupId: group.id },
                  {
                    onSuccess: () => {
                      setPicked([]);
                      setAddOpen(false);
                    },
                  },
                )
              }
            >
              Add {pluralize(picked.length, "member")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <PickerDialog
        open={roleOpen}
        onOpenChange={setRoleOpen}
        title={`Add a role to ${group.name}`}
        fieldLabel="Role"
        options={roles
          .filter((r) => !r.builtIn && !group.roles.includes(r.name))
          .map((r) => ({ value: r.name, label: r.name, description: r.description }))}
        emptyTitle="Every role is already mapped"
        confirmLabel="Add role"
        pending={addRole.isPending}
        onConfirm={(role) =>
          addRole.mutate({ id: group.id, role }, { onSuccess: () => setRoleOpen(false) })
        }
      />

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={(o) => {
          if (!o) setDeleteBlockedBy(null);
          setDeleteOpen(o);
        }}
        title={`Delete ${group.name}?`}
        description={
          deleteBlockedBy ? (
            <span className="text-destructive-foreground">
              This group is still referenced by {deleteBlockedBy.length} service gate(s). Remove it
              from those NebariApps first.
            </span>
          ) : (
            `${pluralize(members.length, "member")} lose the group and everything it unlocks. This cannot be undone.`
          )
        }
        confirmLabel="Delete group"
        destructive
        pending={deleteGroup.isPending}
        onConfirm={() =>
          deleteGroup.mutate(group.id, {
            onSuccess: () => navigate("/admin/groups"),
            onError: (err) => {
              if (err instanceof GroupInUseError) setDeleteBlockedBy(err.services);
            },
          })
        }
      />
    </section>
  );
}
