import { apiFetch } from "@/api/client";
import type { AdminGroup } from "./types";

export class GroupInUseError extends Error {
  readonly services: string[];
  constructor(services: string[]) {
    super("Group is referenced by one or more services");
    this.name = "GroupInUseError";
    this.services = services;
  }
}

async function expectJson<T>(resp: Response): Promise<T> {
  if (!resp.ok) {
    const body = (await resp.json().catch(() => null)) as {
      error?: string;
      services?: string[];
    } | null;
    if (resp.status === 409 && body?.services) throw new GroupInUseError(body.services);
    throw new Error(body?.error ?? `Response: ${resp.status} ${resp.statusText}`);
  }
  return (await resp.json()) as T;
}

export async function listGroups(): Promise<AdminGroup[]> {
  return expectJson(await apiFetch("/admin/groups"));
}

export async function getGroup(id: string): Promise<AdminGroup> {
  return expectJson(await apiFetch(`/admin/groups/${encodeURIComponent(id)}`));
}

export async function createGroup(input: {
  name: string;
  description?: string;
}): Promise<AdminGroup> {
  return expectJson(
    await apiFetch("/admin/groups", { method: "POST", body: JSON.stringify(input) }),
  );
}

export async function updateGroup(
  id: string,
  input: { description?: string },
): Promise<AdminGroup> {
  return expectJson(
    await apiFetch(`/admin/groups/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify(input),
    }),
  );
}

export async function deleteGroup(id: string): Promise<void> {
  const resp = await apiFetch(`/admin/groups/${encodeURIComponent(id)}`, { method: "DELETE" });
  if (resp.status === 204) return;
  await expectJson(resp);
}

export async function addGroupMember(id: string, userId: string): Promise<AdminGroup> {
  return expectJson(
    await apiFetch(
      `/admin/groups/${encodeURIComponent(id)}/members/${encodeURIComponent(userId)}`,
      { method: "PUT" },
    ),
  );
}

export async function removeGroupMember(id: string, userId: string): Promise<AdminGroup> {
  return expectJson(
    await apiFetch(
      `/admin/groups/${encodeURIComponent(id)}/members/${encodeURIComponent(userId)}`,
      { method: "DELETE" },
    ),
  );
}

export async function addGroupRole(id: string, role: string): Promise<AdminGroup> {
  return expectJson(
    await apiFetch(`/admin/groups/${encodeURIComponent(id)}/roles/${encodeURIComponent(role)}`, {
      method: "PUT",
    }),
  );
}

export async function removeGroupRole(id: string, role: string): Promise<AdminGroup> {
  return expectJson(
    await apiFetch(`/admin/groups/${encodeURIComponent(id)}/roles/${encodeURIComponent(role)}`, {
      method: "DELETE",
    }),
  );
}
