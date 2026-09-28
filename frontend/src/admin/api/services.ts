import { apiFetch } from "@/api/client";
import type { AdminService } from "./types";

export async function listAdminServices(): Promise<AdminService[]> {
  const resp = await apiFetch("/admin/services");
  if (!resp.ok) throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  return (await resp.json()) as AdminService[];
}
