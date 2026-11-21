import { apiFetch } from "./client";

export type CallerIdentity = {
  authenticated: boolean;
  username?: string;
  email?: string;
  name?: string;
  groups?: string[];
  /** True when the caller belongs to the webapi's configured admin group. */
  isAdmin: boolean;
};

export async function getCallerIdentity(): Promise<CallerIdentity> {
  const resp = await apiFetch("/caller-identity");

  if (!resp.ok) {
    throw new Error(`Response: ${resp.status} ${resp.statusText}`);
  }

  return resp.json();
}
