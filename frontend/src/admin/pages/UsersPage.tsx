import { ExternalLink, UserMinus, UserPlus } from "lucide-react";
import { useMemo, useState } from "react";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumnDef } from "@/components/ui/data-table";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { AdminUser } from "../api/types";
import { BadgeOverflow, GroupBadge, RoleBadge, StatusBadge } from "../components/EntityBadges";
import { PageHeader } from "../components/PageHeader";
import { PickerDialog } from "../components/PickerDialog";
import { useAdminWorld, useBulkUpdateUsers } from "../hooks/useAdminData";
import { displayName, formatDate, pluralize } from "../lib/format";
import { keycloakConsoleUrl } from "../lib/keycloakConsole";

type Row = AdminUser & Record<string, unknown> & { search: string; groupNames: string[] };

const ALL = "__all__";

export function UsersPage() {
  const { users, groups, roles, services, isLoading, error, refetch } = useAdminWorld();
  const [groupFilter, setGroupFilter] = useState<string>(ALL);
  const [roleFilter, setRoleFilter] = useState<string>(ALL);
  const [statusFilter, setStatusFilter] = useState<string>(ALL);

  const groupName = useMemo(() => new Map(groups.map((g) => [g.id, g.name])), [groups]);

  const rows = useMemo<Row[]>(() => {
    return users
      .filter((u) => groupFilter === ALL || u.groups.includes(groupFilter))
      .filter(
        (u) =>
          roleFilter === ALL ||
          u.roles.includes(roleFilter) ||
          u.groups.some((gid) => groups.find((g) => g.id === gid)?.roles.includes(roleFilter)),
      )
      .filter((u) => statusFilter === ALL || u.enabled === (statusFilter === "enabled"))
      .map((u) => ({
        ...u,
        search: `${displayName(u)} ${u.username} ${u.email}`,
        groupNames: u.groups.map((gid) => groupName.get(gid) ?? gid),
      }));
  }, [users, groups, groupName, groupFilter, roleFilter, statusFilter]);

  const columns = useMemo<DataTableColumnDef<Row>[]>(
    () => [
      {
        id: "search",
        accessorKey: "search",
        header: "User",
        cell: ({ row }) => (
          <Link
            to={`/admin/users/${encodeURIComponent(row.original.id)}`}
            className="flex min-w-0 flex-col rounded-sm outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
          >
            <span className="truncate font-medium text-foreground">
              {displayName(row.original)}
            </span>
            <span className="truncate text-xs text-muted-foreground">{row.original.email}</span>
          </Link>
        ),
      },
      {
        id: "username",
        accessorKey: "username",
        header: "Username",
        cell: ({ row }) => <span className="font-mono text-xs">{row.original.username}</span>,
      },
      {
        id: "groups",
        header: "Groups",
        enableSorting: false,
        cell: ({ row }) => (
          <BadgeOverflow
            items={row.original.groups}
            render={(gid) => <GroupBadge key={gid} id={gid} name={groupName.get(gid) ?? gid} />}
          />
        ),
      },
      {
        id: "roles",
        header: "Direct roles",
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
    ],
    [groupName],
  );

  return (
    <section aria-labelledby="users-title">
      <PageHeader
        title={<span id="users-title">Users</span>}
        description={
          isLoading
            ? "Loading…"
            : `${pluralize(users.length, "account")} across ${pluralize(groups.length, "group")} and ${pluralize(services.length, "service")}.`
        }
        actions={
          <Button
            variant="outline"
            render={
              <a href={keycloakConsoleUrl("/users/add-user")} target="_blank" rel="noreferrer" />
            }
          >
            Create user in Keycloak
            <ExternalLink aria-hidden="true" />
          </Button>
        }
      />

      <DataTable
        ariaLabel="Users"
        columns={columns}
        data={rows}
        getRowId={(r) => r.id}
        getRowLabel={(r) => displayName(r)}
        filterColumnId="search"
        filterLabel="Search users"
        filterPlaceholder="Search name, username or email…"
        initialPageSize={25}
        pageSizeOptions={[10, 25, 50, 100]}
        selectable
        showPagination
        loading={isLoading}
        error={error ? error.message : undefined}
        onRetry={() => void refetch()}
        emptyTitle="No users yet"
        emptyDescription="Users appear here once they sign in through Keycloak or are created in the Keycloak console."
        filteredEmptyTitle="No users match"
        filteredEmptyDescription="Try a different search, or clear the group, role and status filters."
        toolbarActions={
          <div className="flex shrink-0 flex-wrap items-center gap-2">
            <FilterSelect
              label="Group"
              value={groupFilter}
              onChange={setGroupFilter}
              options={groups.map((g) => ({ value: g.id, label: g.name }))}
              allLabel="All groups"
            />
            <FilterSelect
              label="Role"
              value={roleFilter}
              onChange={setRoleFilter}
              options={roles
                .filter((r) => !r.builtIn)
                .map((r) => ({ value: r.name, label: r.name }))}
              allLabel="All roles"
            />
            <FilterSelect
              label="Status"
              value={statusFilter}
              onChange={setStatusFilter}
              options={[
                { value: "enabled", label: "Active" },
                { value: "disabled", label: "Disabled" },
              ]}
              allLabel="Any status"
            />
          </div>
        }
        selectionActions={(selected) => <BulkActions selected={selected} />}
      />
    </section>
  );
}

function FilterSelect({
  label,
  value,
  onChange,
  options,
  allLabel,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
  allLabel: string;
}) {
  const labelFor = (v: string | null) =>
    v === ALL || v === null ? allLabel : (options.find((o) => o.value === v)?.label ?? allLabel);
  return (
    <Select value={value} onValueChange={(v) => onChange((v as string | null) ?? ALL)}>
      <SelectTrigger aria-label={`Filter by ${label.toLowerCase()}`} className="w-40">
        <SelectValue>{(v: string | null) => labelFor(v)}</SelectValue>
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL}>{allLabel}</SelectItem>
        {options.map((o) => (
          <SelectItem key={o.value} value={o.value}>
            {o.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function BulkActions({ selected }: { selected: AdminUser[] }) {
  const { groups, roles } = useAdminWorld();
  const bulk = useBulkUpdateUsers();
  const [dialog, setDialog] = useState<"addToGroup" | "removeFromGroup" | "assignRole" | null>(
    null,
  );
  const ids = selected.map((u) => u.id);

  const groupOptions = groups.map((g) => ({
    value: g.id,
    label: g.name,
    description: g.description,
  }));
  const roleOptions = roles
    .filter((r) => !r.builtIn)
    .map((r) => ({ value: r.name, label: r.name, description: r.description }));

  const anyEnabled = selected.some((u) => u.enabled);
  const anyDisabled = selected.some((u) => !u.enabled);

  return (
    <div className="flex flex-wrap items-center gap-2">
      <Button size="sm" variant="outline" onClick={() => setDialog("addToGroup")}>
        <UserPlus aria-hidden="true" />
        Add to group…
      </Button>
      <Button size="sm" variant="outline" onClick={() => setDialog("removeFromGroup")}>
        <UserMinus aria-hidden="true" />
        Remove from group…
      </Button>
      <Button size="sm" variant="outline" onClick={() => setDialog("assignRole")}>
        Assign role…
      </Button>
      {anyDisabled ? (
        <Button
          size="sm"
          variant="outline"
          loading={bulk.isPending}
          onClick={() => bulk.mutate({ userIds: ids, action: "enable" })}
        >
          Enable
        </Button>
      ) : null}
      {anyEnabled ? (
        <Button
          size="sm"
          variant="destructive"
          loading={bulk.isPending}
          onClick={() => bulk.mutate({ userIds: ids, action: "disable" })}
        >
          Disable
        </Button>
      ) : null}

      <PickerDialog
        open={dialog === "addToGroup"}
        onOpenChange={(o) => !o && setDialog(null)}
        title={`Add ${pluralize(ids.length, "user")} to a group`}
        description="Members gain every service the group unlocks, and inherit the group's roles."
        fieldLabel="Group"
        options={groupOptions}
        confirmLabel="Add to group"
        pending={bulk.isPending}
        onConfirm={(groupId) =>
          bulk.mutate(
            { userIds: ids, action: "addToGroup", groupId },
            { onSuccess: () => setDialog(null) },
          )
        }
      />
      <PickerDialog
        open={dialog === "removeFromGroup"}
        onOpenChange={(o) => !o && setDialog(null)}
        title={`Remove ${pluralize(ids.length, "user")} from a group`}
        description="Access to services gated on this group is revoked immediately for these users."
        fieldLabel="Group"
        options={groupOptions.filter((g) => selected.some((u) => u.groups.includes(g.value)))}
        emptyTitle="None of the selected users are in a group"
        confirmLabel="Remove from group"
        pending={bulk.isPending}
        onConfirm={(groupId) =>
          bulk.mutate(
            { userIds: ids, action: "removeFromGroup", groupId },
            { onSuccess: () => setDialog(null) },
          )
        }
      />
      <PickerDialog
        open={dialog === "assignRole"}
        onOpenChange={(o) => !o && setDialog(null)}
        title={`Assign a role to ${pluralize(ids.length, "user")}`}
        description="Realm roles are mapped directly to each user. Prefer group roles for anything long-lived."
        fieldLabel="Role"
        options={roleOptions}
        confirmLabel="Assign role"
        pending={bulk.isPending}
        onConfirm={(role) =>
          bulk.mutate(
            { userIds: ids, action: "assignRole", role },
            { onSuccess: () => setDialog(null) },
          )
        }
      />
    </div>
  );
}
