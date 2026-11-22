import { useMemo } from "react";
import { getKeycloakInstance } from "./keycloak";

export type User = {
  name: string;
  email: string;
  /** From the name claims only; undefined when the user has no name set. */
  firstName?: string;
};

/**
 * Returns the authenticated user from the Keycloak ID token.
 */
export function useUser(): { user: User | null; loading: boolean } {
  const user = useMemo<User | null>(() => {
    const kc = getKeycloakInstance();
    if (!kc?.authenticated || !kc.idTokenParsed) return null;

    const parsed = kc.idTokenParsed as Record<string, string>;
    const name = parsed.name || parsed.preferred_username || parsed.sub || "User";
    const email = parsed.email || "";
    const firstName = parsed.given_name || parsed.name?.split(/\s+/)[0] || undefined;

    return { name, email, firstName };
  }, []);

  return { user, loading: false };
}

/** Derive up-to-two initials from a display name, e.g. "John Doe" → "JD". */
export function getInitials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0].toUpperCase())
    .join("");
}
