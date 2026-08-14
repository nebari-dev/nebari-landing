import { apiFetch } from "./client";

export type BuildInfo = {
  version: string;
  commit: string;
  lastUpdated: string | null;
};

/**
 * Fetches Nebari Core build metadata from the webapi.
 *
 * This defines the frontend contract for the forthcoming endpoint; the webapi
 * implementation will provide the version, source commit, and update time.
 */
export async function getBuildInfo(signal?: AbortSignal): Promise<BuildInfo> {
  const response = await apiFetch("/build-info", { signal });

  if (!response.ok) {
    throw new Error(`Response: ${response.status} ${response.statusText}`);
  }

  return response.json() as Promise<BuildInfo>;
}
