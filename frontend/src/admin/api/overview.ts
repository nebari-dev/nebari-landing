import { apiFetch } from "@/api/client";
import type { AdminOverview } from "./types";

export async function getAdminOverview(): Promise<AdminOverview> {
  const resp = await apiFetch("/admin/overview");
  if (!resp.ok) throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  return (await resp.json()) as AdminOverview;
}
