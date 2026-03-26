import { useMemo } from "react";
import { getKeycloakInstance } from "./keycloak";

export type User = {
  name: string;
  email: string;
};

const mockUser: User = {
  name: "Test User",
  email: "test.user@example.com",
};

/**
 * Returns the authenticated user from the Keycloak ID token.
 *
 * In bypass mode, returns a local mock user so the app can render without
 * Keycloak or a real authenticated session.
 */
export function useUser(): { user: User | null; loading: boolean } {
  const user = useMemo<User | null>(() => {
    const shouldBypassAuth = import.meta.env.VITE_BYPASS_AUTH === "true";

    if (shouldBypassAuth) {
      return mockUser;
    }

    const kc = getKeycloakInstance();
    if (!kc?.authenticated || !kc.idTokenParsed) return null;

    const parsed = kc.idTokenParsed as Record<string, string>;
    const name =
      parsed["name"] ||
      parsed["preferred_username"] ||
      parsed["sub"] ||
      "User";
    const email = parsed["email"] || "";

    return { name, email };
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
