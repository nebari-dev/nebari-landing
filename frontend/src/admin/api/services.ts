import { apiFetch } from "@/api/client";
import type { AdminService, AdminServiceHealthHistory } from "./types";

export async function listAdminServices(): Promise<AdminService[]> {
  const resp = await apiFetch("/admin/services");
  if (!resp.ok) throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  return (await resp.json()) as AdminService[];
}

export async function getServiceHealth(id: string): Promise<AdminServiceHealthHistory> {
  const resp = await apiFetch(`/admin/services/${encodeURIComponent(id)}/health`);
  if (!resp.ok) throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  return (await resp.json()) as AdminServiceHealthHistory;
}
