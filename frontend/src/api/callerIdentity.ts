import { apiFetch } from "./client";

/** Mirrors `api.CallerIdentityResponse` from the webapi. */
export type CallerIdentity = {
  authenticated: boolean;
  username?: string;
  name?: string;
  email?: string;
  /** Keycloak groups; may arrive as bare names ("admin") or paths ("/admin"). */
  groups?: string[];
};

/**
 * Fetches the server's view of the caller. This is the source of truth for
 * group membership (and therefore admin-ness): the webapi validates the JWT
 * and echoes back the claims it will actually enforce.
 */
export async function getCallerIdentity(): Promise<CallerIdentity> {
  const resp = await apiFetch("/caller-identity");
  if (!resp.ok) throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  return (await resp.json()) as CallerIdentity;
}
