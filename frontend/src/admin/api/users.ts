import { apiFetch } from "@/api/client";
import type { AdminUser, BulkUsersRequest, BulkUsersResponse, UsersListResponse } from "./types";

async function expectJson<T>(resp: Response): Promise<T> {
  if (!resp.ok) {
    const body = (await resp.json().catch(() => null)) as { error?: string } | null;
    throw new Error(body?.error ?? `Response: ${resp.status} ${resp.statusText}`);
  }
  return (await resp.json()) as T;
}

export async function listUsers(): Promise<UsersListResponse> {
  return expectJson(await apiFetch("/admin/users"));
}

export async function getUser(id: string): Promise<AdminUser> {
  return expectJson(await apiFetch(`/admin/users/${encodeURIComponent(id)}`));
}

export async function setUserEnabled(id: string, enabled: boolean): Promise<AdminUser> {
  return expectJson(
    await apiFetch(`/admin/users/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify({ enabled }),
    }),
  );
}

export async function addUserToGroup(id: string, groupId: string): Promise<AdminUser> {
  return expectJson(
    await apiFetch(`/admin/users/${encodeURIComponent(id)}/groups/${encodeURIComponent(groupId)}`, {
      method: "PUT",
    }),
  );
}

export async function removeUserFromGroup(id: string, groupId: string): Promise<AdminUser> {
  return expectJson(
    await apiFetch(`/admin/users/${encodeURIComponent(id)}/groups/${encodeURIComponent(groupId)}`, {
      method: "DELETE",
    }),
  );
}

export async function assignUserRole(id: string, role: string): Promise<AdminUser> {
  return expectJson(
    await apiFetch(`/admin/users/${encodeURIComponent(id)}/roles/${encodeURIComponent(role)}`, {
      method: "PUT",
    }),
  );
}

export async function unassignUserRole(id: string, role: string): Promise<AdminUser> {
  return expectJson(
    await apiFetch(`/admin/users/${encodeURIComponent(id)}/roles/${encodeURIComponent(role)}`, {
      method: "DELETE",
    }),
  );
}

export async function bulkUpdateUsers(req: BulkUsersRequest): Promise<BulkUsersResponse> {
  return expectJson(
    await apiFetch("/admin/users/bulk", { method: "POST", body: JSON.stringify(req) }),
  );
}
