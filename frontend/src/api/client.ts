/**
 * Shared API client for webapi calls.
 *
 * Architecture (production):
 *   Browser → Gateway HTTPRoute → /* → frontend Service (oauth2-proxy)
 *     → oauth2-proxy validates session cookie, injects Authorization: Bearer <token>
 *       → nginx (127.0.0.1:8080)
 *         → /api/* proxy_pass → webapi ClusterIP (cluster DNS, never public)
 *         → /*     try_files  → SPA index.html
 *
 * The browser never communicates with the webapi directly. All auth token
 * forwarding happens server-side: oauth2-proxy → nginx → webapi.
 * The SPA only needs to include the session cookie (credentials: "include").
 *
 * In local dev (Vite dev server), the vite.config.ts proxy forwards /api/*
 * to the webapi running on localhost. Set VITE_WEBAPI_URL to override.
 */

const API_BASE = "/api/v1";

type RequestOptions = Omit<RequestInit, "headers"> & {
    headers?: Record<string, string>;
};

/**
 * Authenticated fetch wrapper.
 *
 * Prepends /api/v1 to the given path and includes the session cookie so that
 * oauth2-proxy can validate the request and forward the Bearer token to the
 * webapi via nginx.
 */
export async function apiFetch(
    path: string,
    options: RequestOptions = {}
): Promise<Response> {
    const url = `${API_BASE}${path.startsWith("/") ? path : `/${path}`}`;

    return fetch(url, {
        credentials: "include",
        ...options,
        headers: {
            "Content-Type": "application/json",
            ...options.headers,
        },
    });
}
