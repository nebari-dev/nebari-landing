import { useQuery } from "@tanstack/react-query";
import { type CallerIdentity, getCallerIdentity } from "@/api/callerIdentity";
import type { User } from "@/auth/user";

/**
 * Name of the Keycloak group whose members may open the admin area. Mirrors
 * the webapi's `webapi.keycloak.adminGroup` default; the server is the
 * authority and rejects admin calls from anyone else regardless of this value.
 */
export const ADMIN_GROUP = "admin";

/** Keycloak may emit group paths ("/admin") or bare names ("admin"). */
export function normalizeGroupName(group: string): string {
  return group.replace(/^\/+/, "");
}

export function isAdminIdentity(identity: CallerIdentity | undefined, adminGroup = ADMIN_GROUP) {
  if (!identity?.authenticated) return false;
  return (identity.groups ?? []).map(normalizeGroupName).includes(adminGroup);
}

/**
 * Server-trusted identity for the signed-in user. Only fetched while a user
 * is signed in; anonymous visitors never hit the endpoint.
 */
export function useCallerIdentity(user: User | null) {
  const query = useQuery({
    queryKey: ["caller-identity"],
    queryFn: getCallerIdentity,
    enabled: user !== null,
    staleTime: 5 * 60 * 1000,
  });

  return {
    identity: query.data,
    groups: (query.data?.groups ?? []).map(normalizeGroupName),
    isAdmin: isAdminIdentity(query.data),
    isLoading: user !== null && query.isPending,
    error: query.error,
  };
}
