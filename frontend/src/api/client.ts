/**
 * Shared API client for webapi calls.
 *
 * Auth: the SPA authenticates directly with Keycloak (PKCE/S256) via keycloak-js
 * and attaches the access token as `Authorization: Bearer <token>` on every
 * request. This works in both dev (no oauth2-proxy) and production (oauth2-proxy
 * may also inject the header server-side — harmless duplication, same token).
 *
 * In local dev (Vite dev server), the vite.config.ts proxy forwards /api/*
 * to the webapi running on localhost. Set VITE_WEBAPI_URL to override.
 */

import { getToken } from "../auth/keycloak";

const API_BASE = "/api/v1";

type RequestOptions = Omit<RequestInit, "headers"> & {
    headers?: Record<string, string>;
};

/**
 * Authenticated fetch wrapper.
 *
 * Prepends /api/v1 to the given path, attaches the current Keycloak access
 * token as `Authorization: Bearer <token>` when one is available, and also
 * includes the session cookie so that oauth2-proxy (when present in production)
 * can independently validate the session.
 *
 * On 401 responses, forces a token refresh and retries once. If the session
 * is fully expired (refresh token gone), keycloak-js redirects to login.
 */
export async function apiFetch(
    path: string,
    options: RequestOptions = {}
): Promise<Response> {
    const url = `${API_BASE}${path.startsWith("/") ? path : `/${path}`}`;

    const doFetch = async () => {
        const token = await getToken();
        return fetch(url, {
            credentials: "include",
            ...options,
            headers: {
                "Content-Type": "application/json",
                Authorization: `Bearer ${token}`,
                ...options.headers,
            },
        });
    };

    const response = await doFetch();

    // On 401, the token may have expired between getToken() and the server
    // receiving it. Force a refresh and retry once.
    if (response.status === 401) {
        return doFetch();
    }

    return response;
}
