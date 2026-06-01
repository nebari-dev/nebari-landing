/**
 * Shared API client for webapi calls.
 *
 * Auth: the SPA authenticates directly with Keycloak (PKCE/S256) via keycloak-js
 * and attaches the access token as `Authorization: Bearer <token>` on every
 * request. This is the sole auth mechanism — there is no server-side session
 * or oauth2-proxy sidecar.
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
 * Prepends /api/v1 to the given path and attaches the current Keycloak access
 * token as `Authorization: Bearer <token>`.
 *
 * On 401 responses, forces a token refresh and retries once. If the session
 * is fully expired (refresh token gone), keycloak-js redirects to login.
 */
export async function apiFetch(path: string, options: RequestOptions = {}): Promise<Response> {
  const url = `${API_BASE}${path.startsWith("/") ? path : `/${path}`}`;

  const doFetch = async () => {
    const token = await getToken();
    return fetch(url, {
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
