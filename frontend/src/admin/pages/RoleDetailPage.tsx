import { Lock, Plus, Trash2, X } from "lucide-react";
import { useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
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
import type { AdminUser } from "../api/types";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { EmptyState } from "../components/EmptyState";
import { GroupBadge, StatusBadge } from "../components/EntityBadges";
import { PageHeader } from "../components/PageHeader";
import { PickerDialog } from "../components/PickerDialog";
import { UserPicker } from "../components/UserPicker";
import {
  useAddGroupRole,
  useAdminWorld,
  useBulkUpdateUsers,
  useDeleteRole,
  useRemoveGroupRole,
  useUnassignUserRole,
} from "../hooks/useAdminData";
import { displayName, pluralize } from "../lib/format";

export function RoleDetailPage() {
  const { name = "" } = useParams();
  const navigate = useNavigate();
  const { users, groups, roles, isLoading } = useAdminWorld();
  const role = roles.find((r) => r.name === name);

  const bulk = useBulkUpdateUsers();
  const unassign = useUnassignUserRole();
  const addGroupRole = useAddGroupRole();
  const removeGroupRole = useRemoveGroupRole();
  const deleteRole = useDeleteRole();

  const [usersOpen, setUsersOpen] = useState(false);
  const [groupOpen, setGroupOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [picked, setPicked] = useState<AdminUser[]>([]);

  const directUsers = useMemo(
    () => (role ? users.filter((u) => u.roles.includes(role.name)) : []),
    [users, role],
  );
  const grantingGroups = useMemo(
    () => (role ? groups.filter((g) => g.roles.includes(role.name)) : []),
    [groups, role],
  );
  const inheritedUsers = useMemo(() => {
    if (!role) return [];
    const ids = new Set(grantingGroups.map((g) => g.id));
    return users.filter(
      (u) => !u.roles.includes(role.name) && u.groups.some((gid) => ids.has(gid)),
    );
  }, [users, role, grantingGroups]);

  if (isLoading) {
    return (
      <output aria-label="Loading role" className="flex flex-col gap-3">
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-8 w-72" />
        <Skeleton className="h-64" shape="block" />
      </output>
    );
  }

  if (!role) {
    return (
      <EmptyState
        title="Role not found"
        action={
          <Button variant="outline" render={<Link to="/admin/roles" />}>
            Back to roles
          </Button>
        }
      />
    );
  }

  const candidates = users.filter((u) => !u.roles.includes(role.name));

  return (
    <section aria-labelledby="role-title">
      <PageHeader
        crumbs={[{ label: "Roles", to: "/admin/roles" }, { label: role.name }]}
        title={
          <span id="role-title" className="flex items-center gap-2">
            {role.name}
            {role.builtIn ? (
              <Badge variant="ghost" className="text-muted-foreground">
                <Lock aria-hidden="true" />
                built-in
              </Badge>
            ) : null}
            {role.composite ? <Badge variant="outline">composite</Badge> : null}
          </span>
        }
        description={role.description || "No description"}
        actions={
          <>
            <Button onClick={() => setUsersOpen(true)}>
              <Plus aria-hidden="true" />
              Assign to users
            </Button>
            <Button variant="outline" onClick={() => setGroupOpen(true)}>
              <Plus aria-hidden="true" />
              Map to group
            </Button>
            {!role.builtIn ? (
              <Button variant="outline" onClick={() => setDeleteOpen(true)}>
                <Trash2 aria-hidden="true" />
                Delete
              </Button>
            ) : null}
          </>
        }
      />

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Groups that grant this role ({grantingGroups.length})</CardTitle>
            <CardDescription>
              Preferred way to hand out a role: members inherit it and lose it when they leave.
            </CardDescription>
          </CardHeader>
          <CardContent>
            {grantingGroups.length === 0 ? (
              <EmptyState
                title="No group maps this role"
                action={
                  <Button size="sm" variant="outline" onClick={() => setGroupOpen(true)}>
                    Map to group
                  </Button>
                }
              />
            ) : (
              <ul className="divide-y divide-border">
                {grantingGroups.map((g) => (
                  <li key={g.id} className="flex items-center justify-between gap-3 py-2">
                    <div>
                      <GroupBadge id={g.id} name={g.name} />
                      <p className="mt-1 text-xs text-muted-foreground">
                        {pluralize(users.filter((u) => u.groups.includes(g.id)).length, "member")}
                      </p>
                    </div>
                    <Button
                      size="sm"
                      variant="ghost"
                      aria-label={`Unmap from ${g.name}`}
                      onClick={() => removeGroupRole.mutate({ id: g.id, role: role.name })}
                    >
                      <X aria-hidden="true" />
                      Unmap
                    </Button>
                  </li>
                ))}
              </ul>
            )}
            {inheritedUsers.length > 0 ? (
              <p className="pt-3 text-xs text-muted-foreground">
                {pluralize(inheritedUsers.length, "user")} inherit this role through these groups.
              </p>
            ) : null}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Users with a direct mapping ({directUsers.length})</CardTitle>
            <CardDescription>Assigned individually, outside any group.</CardDescription>
          </CardHeader>
          <CardContent>
            {directUsers.length === 0 ? (
              <EmptyState title="No direct assignments" />
            ) : (
              <ul className="divide-y divide-border">
                {directUsers.map((u) => (
                  <li key={u.id} className="flex items-center justify-between gap-3 py-2">
                    <div className="min-w-0">
                      <Link
                        to={`/admin/users/${encodeURIComponent(u.id)}`}
                        className="font-medium hover:underline"
                      >
                        {displayName(u)}
                      </Link>
                      <span className="ml-2 align-middle">
                        <StatusBadge enabled={u.enabled} />
                      </span>
                      <p className="truncate text-xs text-muted-foreground">{u.email}</p>
                    </div>
                    <Button
                      size="sm"
                      variant="ghost"
                      aria-label={`Remove role from ${displayName(u)}`}
                      onClick={() => unassign.mutate({ id: u.id, role: role.name })}
                    >
                      <X aria-hidden="true" />
                      Remove
                    </Button>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>

      <Dialog
        open={usersOpen}
        onOpenChange={(o) => {
          if (!o) setPicked([]);
          setUsersOpen(o);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Assign {role.name} to users</DialogTitle>
            <DialogDescription>
              Direct mappings are easy to forget. Prefer mapping the role to a group when more than
              a few people need it.
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
                  { userIds: picked.map((u) => u.id), action: "assignRole", role: role.name },
                  {
                    onSuccess: () => {
                      setPicked([]);
                      setUsersOpen(false);
                    },
                  },
                )
              }
            >
              Assign to {pluralize(picked.length, "user")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <PickerDialog
        open={groupOpen}
        onOpenChange={setGroupOpen}
        title={`Map ${role.name} to a group`}
        fieldLabel="Group"
        options={groups
          .filter((g) => !g.roles.includes(role.name))
          .map((g) => ({ value: g.id, label: g.name, description: g.description }))}
        emptyTitle="Every group already grants this role"
        confirmLabel="Map to group"
        pending={addGroupRole.isPending}
        onConfirm={(groupId) =>
          addGroupRole.mutate(
            { id: groupId, role: role.name },
            { onSuccess: () => setGroupOpen(false) },
          )
        }
      />

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={`Delete ${role.name}?`}
        description={`It is removed from ${pluralize(directUsers.length, "user")} and ${pluralize(grantingGroups.length, "group")}. Packs that expect this role will stop granting the matching permission.`}
        confirmLabel="Delete role"
        destructive
        pending={deleteRole.isPending}
        onConfirm={() =>
          deleteRole.mutate(role.name, { onSuccess: () => navigate("/admin/roles") })
        }
      />
    </section>
  );
}
