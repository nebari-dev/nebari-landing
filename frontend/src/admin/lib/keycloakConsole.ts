import { getAppConfig } from "@/app/config";

/**
 * Deep link into the Keycloak admin console for the realm the SPA signs in
 * against. Uses the browser-facing Keycloak URL from /config.json so the link
 * works wherever the Launchpad is deployed.
 */
export function keycloakConsoleUrl(path = ""): string {
  const kc = getAppConfig()?.keycloak;
  const base = (kc?.url ?? "").replace(/\/+$/, "");
  const realm = encodeURIComponent(kc?.realm ?? "master");
  return `${base}/admin/${realm}/console/#/${realm}${path}`;
}
