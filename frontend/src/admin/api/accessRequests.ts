import { apiFetch } from "@/api/client";
import type { AccessRequest, AccessRequestStatus } from "./types";

export async function listAccessRequests(status?: AccessRequestStatus): Promise<AccessRequest[]> {
  const qs = status ? `?status=${encodeURIComponent(status)}` : "";
  const resp = await apiFetch(`/admin/access-requests${qs}`);
  if (resp.status === 501) return [];
  if (!resp.ok) throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  const json = (await resp.json()) as AccessRequest[] | { accessRequests: AccessRequest[] };
  return Array.isArray(json) ? json : (json.accessRequests ?? []);
}

export async function resolveAccessRequest(
  id: string,
  action: "approve" | "deny",
): Promise<AccessRequest> {
  const resp = await apiFetch(`/admin/access-requests/${encodeURIComponent(id)}/${action}`, {
    method: "PUT",
  });
  if (!resp.ok) {
    const text = await resp.text().catch(() => "");
    throw new Error(text || `Response: ${resp.status} ${resp.statusText}`);
  }
  return (await resp.json()) as AccessRequest;
}
