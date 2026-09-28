import { apiFetch } from "@/api/client";
import type { AdminRole } from "./types";

async function expectJson<T>(resp: Response): Promise<T> {
  if (!resp.ok) {
    const body = (await resp.json().catch(() => null)) as { error?: string } | null;
    throw new Error(body?.error ?? `Response: ${resp.status} ${resp.statusText}`);
  }
  return (await resp.json()) as T;
}

export async function listRoles(): Promise<AdminRole[]> {
  return expectJson(await apiFetch("/admin/roles"));
}

export async function createRole(input: {
  name: string;
  description?: string;
}): Promise<AdminRole> {
  return expectJson(
    await apiFetch("/admin/roles", { method: "POST", body: JSON.stringify(input) }),
  );
}

export async function updateRole(
  name: string,
  input: { description?: string },
): Promise<AdminRole> {
  return expectJson(
    await apiFetch(`/admin/roles/${encodeURIComponent(name)}`, {
      method: "PATCH",
      body: JSON.stringify(input),
    }),
  );
}

export async function deleteRole(name: string): Promise<void> {
  const resp = await apiFetch(`/admin/roles/${encodeURIComponent(name)}`, { method: "DELETE" });
  if (resp.status === 204) return;
  await expectJson(resp);
}
