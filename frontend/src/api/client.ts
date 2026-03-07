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
 */
export async function apiFetch(
    path: string,
    options: RequestOptions = {}
): Promise<Response> {
    const url = `${API_BASE}${path.startsWith("/") ? path : `/${path}`}`;

    const token = await getToken();
    const authHeader: Record<string, string> = token
        ? { Authorization: `Bearer ${token}` }
        : {};

    return fetch(url, {
        credentials: "include",
        ...options,
        headers: {
            "Content-Type": "application/json",
            ...authHeader,
            ...options.headers,
        },
    });
}
