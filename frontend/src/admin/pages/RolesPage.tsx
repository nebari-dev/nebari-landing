import { Lock, Plus } from "lucide-react";
import { useId, useMemo, useState } from "react";
import { Link } from "react-router";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import type { AdminRole } from "../api/types";
import { BadgeOverflow, GroupBadge } from "../components/EntityBadges";
import { PageHeader } from "../components/PageHeader";
import { useAdminWorld, useCreateRole } from "../hooks/useAdminData";

type Row = AdminRole & Record<string, unknown> & { directUsers: number; groupIds: string[] };

export function RolesPage() {
  const { users, groups, roles, isLoading, error, refetch } = useAdminWorld();
  const [createOpen, setCreateOpen] = useState(false);
  const groupName = useMemo(() => new Map(groups.map((g) => [g.id, g.name])), [groups]);

  const rows = useMemo<Row[]>(
    () =>
      roles.map((r) => ({
        ...r,
        directUsers: users.filter((u) => u.roles.includes(r.name)).length,
        groupIds: groups.filter((g) => g.roles.includes(r.name)).map((g) => g.id),
      })),
    [roles, users, groups],
  );

  const columns = useMemo<DataTableColumnDef<Row>[]>(
    () => [
      {
        id: "name",
        accessorKey: "name",
        header: "Role",
        cell: ({ row }) => (
          <Link
            to={`/admin/roles/${encodeURIComponent(row.original.name)}`}
            className="flex min-w-0 flex-col rounded-sm outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
          >
            <span className="flex items-center gap-2 font-medium text-foreground">
              {row.original.name}
              {row.original.builtIn ? (
                <Badge variant="ghost" className="text-muted-foreground">
                  <Lock aria-hidden="true" />
                  built-in
                </Badge>
              ) : null}
              {row.original.composite ? <Badge variant="outline">composite</Badge> : null}
            </span>
            <span className="truncate text-xs text-muted-foreground">
              {row.original.description || "No description"}
            </span>
          </Link>
        ),
      },
      {
        id: "groups",
        header: "Granted by groups",
        enableSorting: false,
        cell: ({ row }) =>
          row.original.groupIds.length === 0 ? (
            <span className="text-xs text-muted-foreground">—</span>
          ) : (
            <BadgeOverflow
              items={row.original.groupIds}
              render={(gid) => <GroupBadge key={gid} id={gid} name={groupName.get(gid) ?? gid} />}
            />
          ),
      },
      {
        id: "directUsers",
        accessorKey: "directUsers",
        header: "Direct users",
        cell: ({ row }) => <span className="tabular-nums">{row.original.directUsers}</span>,
      },
    ],
    [groupName],
  );

  return (
    <section aria-labelledby="roles-title">
      <PageHeader
        title={<span id="roles-title">Roles</span>}
        description="Realm roles carry pack-level permissions (for example Superset dashboard editing). They do not gate services; groups do."
        actions={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus aria-hidden="true" />
            New role
          </Button>
        }
      />

      <DataTable
        ariaLabel="Roles"
        columns={columns}
        data={rows}
        getRowId={(r) => r.name}
        getRowLabel={(r) => r.name}
        filterColumnId="name"
        filterLabel="Search roles"
        filterPlaceholder="Search roles…"
        initialPageSize={25}
        showPagination={rows.length > 25}
        loading={isLoading}
        error={error ? error.message : undefined}
        onRetry={() => void refetch()}
        emptyTitle="No roles yet"
        emptyDescription="Create a role and map it to a group so members pick it up automatically."
        emptyAction={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus aria-hidden="true" />
            New role
          </Button>
        }
        filteredEmptyTitle="No roles match"
        filteredEmptyDescription="Try a different search."
      />

      <CreateRoleDialog open={createOpen} onOpenChange={setCreateOpen} />
    </section>
  );
}

function CreateRoleDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const create = useCreateRole();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const nameId = useId();
  const descId = useId();

  function reset() {
    setName("");
    setDescription("");
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) reset();
        onOpenChange(o);
      }}
    >
      <DialogContent>
        <form
          className="contents"
          onSubmit={(e) => {
            e.preventDefault();
            create.mutate(
              { name, description },
              {
                onSuccess: () => {
                  reset();
                  onOpenChange(false);
                },
              },
            );
          }}
        >
          <DialogHeader>
            <DialogTitle>New realm role</DialogTitle>
            <DialogDescription>
              Packs read roles from the token to decide in-app permissions. Check the pack's docs
              for the role names it understands.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-2">
            <Field>
              <FieldLabel htmlFor={nameId}>Name</FieldLabel>
              <Input
                id={nameId}
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. dashboard-viewer"
                autoComplete="off"
                required
              />
              <FieldDescription>Lowercase letters, digits and dashes.</FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor={descId}>Description</FieldLabel>
              <Textarea
                id={descId}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="What this role permits"
                rows={3}
              />
            </Field>
          </div>
          <DialogFooter>
            <DialogClose render={<Button type="button" variant="outline" />}>Cancel</DialogClose>
            <Button
              render={<button type="submit" />}
              loading={create.isPending}
              disabled={name.trim() === ""}
            >
              Create role
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
