import { apiFetch } from "@/api/client";
import type { AdminPacksResponse, Pack } from "./types";

export async function listPacks(): Promise<AdminPacksResponse> {
  const resp = await apiFetch("/admin/packs");
  if (!resp.ok) throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  return (await resp.json()) as AdminPacksResponse;
}

export async function getPack(name: string): Promise<Pack> {
  const resp = await apiFetch(`/admin/packs/${encodeURIComponent(name)}`);
  if (!resp.ok) throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  return (await resp.json()) as Pack;
}
