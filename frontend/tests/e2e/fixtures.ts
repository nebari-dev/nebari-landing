/**
 * fixtures.ts - Playwright route interceptors for the mock-chromium project.
 *
 * Two focused helpers intercept only the calls the app makes:
 *   - interceptAuth    - stubs src/auth/keycloak.ts so the app starts
 *                        pre-authenticated without any OIDC redirect
 *   - interceptApiData - stubs /api/v1/* webapi endpoints with mock data
 */

import { test as base, expect, type Page } from "@playwright/test";

// Stub module served in place of src/auth/keycloak.ts.
// Exports the same surface as the real module so the app never initiates
// the OIDC flow and sees itself as already authenticated.
const KC_STUB = [
    "const _kc = {",
    "  authenticated: true,",
    "  idTokenParsed: {",
    '    sub: "user-mock-123",',
    '    name: "Test User",',
    '    email: "test.user@example.com",',
    '    preferred_username: "testuser",',
    "  },",
    "};",
    "export async function initKeycloak() { return _kc; }",
    "export function getKeycloakInstance() { return _kc; }",
    'export async function getToken() { return "mock-token"; }',
    "export function signIn() {}",
    "export function signOut() {}",
].join("\n");

async function interceptAuth(page: Page): Promise<void> {
    // Intercept Vite's request for the keycloak module (path may include ?t= cache-bust).
    await page.route(/\/src\/auth\/keycloak\.ts/, (route) =>
        route.fulfill({ contentType: "application/javascript", body: KC_STUB })
    );
}

async function interceptApiData(page: Page): Promise<void> {
    await page.route("/api/v1/services", (route) =>
        route.fulfill({
            json: {
                services: [
                    { id: "svc-1", name: "JupyterHub", status: "Healthy", description: "Notebook platform", category: ["Data Science"], pinned: true, image: "", url: "https://example.com/jupyterhub" },
                    { id: "svc-2", name: "Grafana", status: "Unhealthy", description: "Metrics dashboards", category: ["Monitoring"], pinned: false, image: "", url: "https://example.com/grafana" },
                    { id: "svc-3", name: "Admin Panel", status: "Unknown", description: "Administrative tools", category: ["Platform"], pinned: false, image: "", url: "https://example.com/admin" },
                ],
            },
        })
    );

    await page.route("/api/v1/notifications", (route) =>
        route.fulfill({
            json: {
                notifications: [
                    { id: "notif-1", image: "", title: "JupyterHub is back online!", message: "JupyterHub is back online and ready to use.", read: false, createdAt: new Date().toISOString() },
                    { id: "notif-2", image: "", title: "Scheduled maintenance planned", message: "Maintenance will occur on the first Saturday of each month.", read: true, createdAt: new Date().toISOString() },
                ],
            },
        })
    );

    await page.route(/\/api\/v1\/(notifications\/.+\/read|pins\/.+)/, (route) =>
        route.fulfill({ status: 204 })
    );
}

export const test = base.extend({
    page: async ({ page }, use) => {
        await interceptAuth(page);
        await interceptApiData(page);
        await use(page); // eslint-disable-line react-hooks/rules-of-hooks
    },
});

export { expect };
