// TanStack Query wrappers for the admin endpoints. Every mutation invalidates
// the whole admin namespace: the entities are small and tightly coupled
// (a group change alters user rows, role rows and the access map), so a
// blanket refetch is simpler and safer than surgical cache edits.

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "@/components/ui/toast";
import * as groupsApi from "../api/groups";
import * as rolesApi from "../api/roles";
import { listAdminServices } from "../api/services";
import type { BulkUsersRequest } from "../api/types";
import * as usersApi from "../api/users";

export const adminKeys = {
  all: ["admin"] as const,
  users: () => [...adminKeys.all, "users"] as const,
  groups: () => [...adminKeys.all, "groups"] as const,
  roles: () => [...adminKeys.all, "roles"] as const,
  services: () => [...adminKeys.all, "services"] as const,
};

/**
 * Keycloak has no change feed, so admin queries refresh on their own: they
 * refetch when the window regains focus (e.g. coming back from the Keycloak
 * console after creating a user) and poll while an admin page is open.
 * Data is served from cache meanwhile, so navigation never blocks.
 */
const adminQueryOptions = {
  staleTime: 15 * 1000,
  refetchOnWindowFocus: true,
  refetchOnReconnect: true,
  refetchInterval: 60 * 1000,
} as const;

export function useAdminUsers() {
  return useQuery({
    ...adminQueryOptions,
    queryKey: adminKeys.users(),
    queryFn: usersApi.listUsers,
  });
}

export function useAdminGroups() {
  return useQuery({
    ...adminQueryOptions,
    queryKey: adminKeys.groups(),
    queryFn: groupsApi.listGroups,
  });
}

export function useAdminRoles() {
  return useQuery({
    ...adminQueryOptions,
    queryKey: adminKeys.roles(),
    queryFn: rolesApi.listRoles,
  });
}

export function useAdminServices() {
  return useQuery({
    ...adminQueryOptions,
    queryKey: adminKeys.services(),
    queryFn: listAdminServices,
  });
}

/** Loads every admin entity at once; detail pages need all four to explain access. */
export function useAdminWorld() {
  const users = useAdminUsers();
  const groups = useAdminGroups();
  const roles = useAdminRoles();
  const services = useAdminServices();
  const queries = [users, groups, roles, services];
  return {
    users: users.data?.users ?? [],
    groups: groups.data ?? [],
    roles: roles.data ?? [],
    services: services.data ?? [],
    isLoading: queries.some((q) => q.isPending),
    error: queries.find((q) => q.error)?.error ?? null,
    refetch: () => Promise.all(queries.map((q) => q.refetch())),
  };
}

type MutationOpts = { success?: string };

function useAdminMutation<TArgs, TResult>(
  fn: (args: TArgs) => Promise<TResult>,
  opts: MutationOpts = {},
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: adminKeys.all });
      if (opts.success) toast.add({ type: "success", title: opts.success });
    },
    onError: (err) => {
      toast.add({ type: "error", title: "Change failed", description: err.message });
    },
  });
}

// --- users ----------------------------------------------------------------

export function useSetUserEnabled() {
  return useAdminMutation(
    ({ id, enabled }: { id: string; enabled: boolean }) => usersApi.setUserEnabled(id, enabled),
    { success: "User updated" },
  );
}

export function useDeleteUser() {
  return useAdminMutation((id: string) => usersApi.deleteUser(id), { success: "User deleted" });
}

export function useAddUserToGroup() {
  return useAdminMutation(
    ({ id, groupId }: { id: string; groupId: string }) => usersApi.addUserToGroup(id, groupId),
    { success: "Added to group" },
  );
}

export function useRemoveUserFromGroup() {
  return useAdminMutation(
    ({ id, groupId }: { id: string; groupId: string }) => usersApi.removeUserFromGroup(id, groupId),
    { success: "Removed from group" },
  );
}

export function useAssignUserRole() {
  return useAdminMutation(
    ({ id, role }: { id: string; role: string }) => usersApi.assignUserRole(id, role),
    { success: "Role assigned" },
  );
}

export function useUnassignUserRole() {
  return useAdminMutation(
    ({ id, role }: { id: string; role: string }) => usersApi.unassignUserRole(id, role),
    { success: "Role removed" },
  );
}

export function useBulkUpdateUsers() {
  return useAdminMutation((req: BulkUsersRequest) => usersApi.bulkUpdateUsers(req), {
    success: "Users updated",
  });
}

// --- groups ---------------------------------------------------------------

export function useCreateGroup() {
  return useAdminMutation(groupsApi.createGroup, { success: "Group created" });
}

export function useUpdateGroup() {
  return useAdminMutation(
    ({ id, description }: { id: string; description: string }) =>
      groupsApi.updateGroup(id, { description }),
    { success: "Group updated" },
  );
}

export function useDeleteGroup() {
  return useAdminMutation((id: string) => groupsApi.deleteGroup(id), {
    success: "Group deleted",
  });
}

export function useAddGroupMember() {
  return useAdminMutation(
    ({ id, userId }: { id: string; userId: string }) => groupsApi.addGroupMember(id, userId),
    { success: "Member added" },
  );
}

export function useRemoveGroupMember() {
  return useAdminMutation(
    ({ id, userId }: { id: string; userId: string }) => groupsApi.removeGroupMember(id, userId),
    { success: "Member removed" },
  );
}

export function useAddGroupRole() {
  return useAdminMutation(
    ({ id, role }: { id: string; role: string }) => groupsApi.addGroupRole(id, role),
    { success: "Role added to group" },
  );
}

export function useRemoveGroupRole() {
  return useAdminMutation(
    ({ id, role }: { id: string; role: string }) => groupsApi.removeGroupRole(id, role),
    { success: "Role removed from group" },
  );
}

// --- roles ----------------------------------------------------------------

export function useCreateRole() {
  return useAdminMutation(rolesApi.createRole, { success: "Role created" });
}

export function useUpdateRole() {
  return useAdminMutation(
    ({ name, description }: { name: string; description: string }) =>
      rolesApi.updateRole(name, { description }),
    { success: "Role updated" },
  );
}

export function useDeleteRole() {
  return useAdminMutation((name: string) => rolesApi.deleteRole(name), {
    success: "Role deleted",
  });
}
