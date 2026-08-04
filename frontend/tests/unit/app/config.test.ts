import { afterEach, describe, expect, it, vi } from "vitest";

// loadAppConfig() caches its result at module scope and only fetches once, so
// each case resets the module registry and re-imports a fresh copy.
async function loadWith(config: Record<string, unknown>) {
  vi.resetModules();
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response(JSON.stringify(config), { status: 200 })),
  );
  const { loadAppConfig } = await import("@/app/config");
  return loadAppConfig();
}

const baseConfig = {
  keycloak: { url: "http://localhost", realm: "nebari", clientId: "spa" },
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("loadAppConfig faviconUrl sanitization", () => {
  it("preserves a base64 image data URI", async () => {
    const favicon = "data:image/png;base64,iVBORw0KGgo=";
    const config = await loadWith({ ...baseConfig, faviconUrl: favicon });
    expect(config.faviconUrl).toBe(favicon);
  });

  it("preserves http(s) and root-relative favicons", async () => {
    expect(
      (await loadWith({ ...baseConfig, faviconUrl: "https://example.com/f.ico" }))
        .faviconUrl,
    ).toBe("https://example.com/f.ico");
    expect(
      (await loadWith({ ...baseConfig, faviconUrl: "/favicon.ico" })).faviconUrl,
    ).toBe("/favicon.ico");
  });

  it("drops a javascript: favicon", async () => {
    const config = await loadWith({ ...baseConfig, faviconUrl: "javascript:alert(1)" });
    expect(config.faviconUrl).toBeUndefined();
  });

  it("drops a data:text/html favicon", async () => {
    const config = await loadWith({
      ...baseConfig,
      faviconUrl: "data:text/html;base64,PHNjcmlwdD4=",
    });
    expect(config.faviconUrl).toBeUndefined();
  });

  it("drops a malformed favicon", async () => {
    const config = await loadWith({ ...baseConfig, faviconUrl: "not a url" });
    expect(config.faviconUrl).toBeUndefined();
  });
});
