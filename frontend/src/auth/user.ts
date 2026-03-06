import { useState, useEffect } from "react";

export type User = {
    name: string;
    email: string;
};

/**
 * Fetches the authenticated user from oauth2-proxy's /oauth2/userinfo endpoint.
 *
 * oauth2-proxy injects the authenticated user's claims as JSON at this endpoint
 * when a valid session cookie is present. A 401/403 response means the user is
 * not yet logged in.
 *
 * Typical Keycloak response shape:
 *   { user: "john.doe@example.com", email: "...", preferredUsername: "john.doe",
 *     name: "John Doe", groups: [...] }
 */
export function useUser(): { user: User | null; loading: boolean } {
    const [user, setUser] = useState<User | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        fetch("/oauth2/userinfo", { credentials: "include" })
            .then((r) => (r.ok ? r.json() : Promise.reject()))
            .then((data: Record<string, string>) => {
                const name =
                    data["name"] ||
                    data["preferredUsername"] ||
                    data["user"] ||
                    "User";
                const email = data["email"] || "";
                setUser({ name, email });
            })
            .catch(() => setUser(null))
            .finally(() => setLoading(false));
    }, []);

    return { user, loading };
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
