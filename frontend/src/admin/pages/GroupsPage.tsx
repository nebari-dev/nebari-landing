import { Plus } from "lucide-react";
import { useId, useMemo, useState } from "react";
import { Link } from "react-router";
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
import type { AdminGroup } from "../api/types";
import { BadgeOverflow, RoleBadge } from "../components/EntityBadges";
import { PageHeader } from "../components/PageHeader";
import { useAdminWorld, useCreateGroup } from "../hooks/useAdminData";
import { servicesUnlockedByGroup } from "../lib/access";
import { pluralize } from "../lib/format";

type Row = AdminGroup & Record<string, unknown> & { memberCount: number; serviceNames: string[] };

export function GroupsPage() {
  const { users, groups, services, isLoading, error, refetch } = useAdminWorld();
  const [createOpen, setCreateOpen] = useState(false);

  const rows = useMemo<Row[]>(
    () =>
      groups.map((g) => ({
        ...g,
        memberCount: users.filter((u) => u.groups.includes(g.id)).length,
        serviceNames: servicesUnlockedByGroup(g, services).map((s) => s.displayName),
      })),
    [groups, users, services],
  );

  const columns = useMemo<DataTableColumnDef<Row>[]>(
    () => [
      {
        id: "name",
        accessorKey: "name",
        header: "Group",
        cell: ({ row }) => (
          <Link
            to={`/admin/groups/${encodeURIComponent(row.original.id)}`}
            className="flex min-w-0 flex-col rounded-sm outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
          >
            <span className="font-medium text-foreground">{row.original.name}</span>
            <span className="truncate text-xs text-muted-foreground">
              {row.original.description || row.original.path}
            </span>
          </Link>
        ),
      },
      {
        id: "memberCount",
        accessorKey: "memberCount",
        header: "Members",
        cell: ({ row }) => <span className="tabular-nums">{row.original.memberCount}</span>,
      },
      {
        id: "roles",
        header: "Grants roles",
        enableSorting: false,
        cell: ({ row }) =>
          row.original.roles.length === 0 ? (
            <span className="text-xs text-muted-foreground">—</span>
          ) : (
            <BadgeOverflow
              items={row.original.roles}
              render={(r) => <RoleBadge key={r} name={r} />}
            />
          ),
      },
      {
        id: "services",
        header: "Unlocks services",
        enableSorting: false,
        cell: ({ row }) =>
          row.original.serviceNames.length === 0 ? (
            <span className="text-xs text-muted-foreground">None</span>
          ) : (
            <span className="text-sm">{row.original.serviceNames.join(", ")}</span>
          ),
      },
    ],
    [],
  );

  return (
    <section aria-labelledby="groups-title">
      <PageHeader
        title={<span id="groups-title">Groups</span>}
        description="Groups are how access is granted: a service lists the groups that may reach it, and members of those groups get in."
        actions={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus aria-hidden="true" />
            New group
          </Button>
        }
      />

      <DataTable
        ariaLabel="Groups"
        columns={columns}
        data={rows}
        getRowId={(r) => r.id}
        getRowLabel={(r) => r.name}
        filterColumnId="name"
        filterLabel="Search groups"
        filterPlaceholder="Search groups…"
        initialPageSize={25}
        showPagination={rows.length > 25}
        loading={isLoading}
        error={error ? error.message : undefined}
        onRetry={() => void refetch()}
        emptyTitle="No groups yet"
        emptyDescription="Create a group, then list it under spec.auth.groups on a NebariApp to gate that service on it."
        emptyAction={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus aria-hidden="true" />
            New group
          </Button>
        }
        filteredEmptyTitle="No groups match"
        filteredEmptyDescription="Try a different search."
      />

      <CreateGroupDialog open={createOpen} onOpenChange={setCreateOpen} />
    </section>
  );
}

function CreateGroupDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const create = useCreateGroup();
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
            <DialogTitle>New group</DialogTitle>
            <DialogDescription>
              Creates a top-level Keycloak group. It gates nothing until a NebariApp lists it.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-2">
            <Field>
              <FieldLabel htmlFor={nameId}>Name</FieldLabel>
              <Input
                id={nameId}
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. data-engineering"
                autoComplete="off"
                required
              />
              <FieldDescription>
                Lowercase letters, digits and dashes. This is the value that appears in the JWT
                groups claim.
              </FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor={descId}>Description</FieldLabel>
              <Textarea
                id={descId}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Who belongs here and why"
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
              Create group
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function groupSummary(memberCount: number, serviceCount: number) {
  return `${pluralize(memberCount, "member")} · unlocks ${pluralize(serviceCount, "service")}`;
}
