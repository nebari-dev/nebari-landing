import { apiFetch } from "./client";

/** Latest probe result; `degraded` and `down` both count as an unhealthy `status`. */
export type HealthState = "healthy" | "degraded" | "down" | "unknown";

export type Service = {
  id: string;
  name: string;
  status: string;
  health?: HealthState;
  description: string;
  category: string[];
  pinned: boolean;
  image?: string;
  imageLight?: string;
  imageDark?: string;
  url: string;
  version?: string;
  /** Always an absolute http(s) URL. */
  docsUrl?: string;
  /** Reserved for the signed-in user's running sessions; the webapi does not send it yet. */
  sessionCount?: number;
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
