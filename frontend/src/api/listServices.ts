import { apiFetch } from "./client";

export type Service = {
  id: string;
  name: string;
  status: string;
  description: string;
  category: string[];
  pinned: boolean;
  image: string;
};

/**
 * Fetches all available services from the webapi.
 *
 * Uses a relative URL (/api/v1/services) so the request is proxied by nginx
 * inside the pod — no cross-origin request, no CORS issues.
 */
export async function listServices(): Promise<Service[]> {
  const resp = await apiFetch("/services");

  if (!resp.ok) {
    throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  }

  // webapi wraps the array: { "services": [...] }
  const json = await resp.json();
  return Array.isArray(json) ? json : (json.services ?? []);
}
