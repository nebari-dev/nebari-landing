import { expect, test } from "./fixtures/e2e";

// A deployer who rebrands `primary` must not get the built-in Nebari magenta
// leaking back in through tokens that were hardcoded instead of derived.
// See frontend/src/app/index.css: --primary-hover and --sidebar-ring.

const BRAND_PRIMARY = "#0066cc";

/**
 * Resolve a CSS custom property to concrete sRGB channels by painting it.
 * getComputedStyle() leaves custom properties as unevaluated token streams,
 * so color-mix() only collapses to a real color once it is used as a color.
 */
async function readTokenRgb(
  page: import("@playwright/test").Page,
  token: string,
): Promise<[number, number, number]> {
  return page.evaluate((name) => {
    const probe = document.createElement("div");
    probe.style.backgroundColor = `var(${name})`;
    document.body.appendChild(probe);
    const painted = getComputedStyle(probe).backgroundColor;
    probe.remove();

    const canvas = document.createElement("canvas");
    canvas.width = 1;
    canvas.height = 1;
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("2d context unavailable");
    ctx.fillStyle = painted;
    ctx.fillRect(0, 0, 1, 1);
    const [r, g, b] = ctx.getImageData(0, 0, 1, 1).data;
    return [r, g, b] as [number, number, number];
  }, token);
}

/** sRGB hue in degrees, matching the HSL definition. */
function hue([r, g, b]: [number, number, number]): number {
  const [rn, gn, bn] = [r / 255, g / 255, b / 255];
  const max = Math.max(rn, gn, bn);
  const min = Math.min(rn, gn, bn);
  const delta = max - min;
  if (delta === 0) return 0;
  let h: number;
  if (max === rn) h = ((gn - bn) / delta) % 6;
  else if (max === gn) h = (bn - rn) / delta + 2;
  else h = (rn - gn) / delta + 4;
  return (h * 60 + 360) % 360;
}

/** Relative luminance, good enough to compare two shades of one hue. */
function luminance([r, g, b]: [number, number, number]): number {
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

test.describe("runtime branding covers derived tokens", () => {
  test.beforeEach(async ({ page }) => {
    await page.route("**/config.json", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          keycloak: {
            url: "http://localhost:8180",
            realm: "nebari",
            clientId: "nebari-frontend-spa",
          },
          theme: {
            light: { primary: BRAND_PRIMARY, ring: BRAND_PRIMARY },
            dark: { primary: BRAND_PRIMARY, ring: BRAND_PRIMARY },
          },
        }),
      });
    });
  });

  for (const mode of ["light", "dark"] as const) {
    test(`--primary-hover follows a rebranded primary in ${mode} mode`, async ({ page }) => {
      await page.addInitScript(
        (themeMode) => window.localStorage.setItem("launchpad:themeMode", themeMode),
        mode,
      );
      await page.goto("/");
      await expect(page.getByRole("button", { name: /account menu/i })).toBeVisible();

      const primary = await readTokenRgb(page, "--primary");
      const primaryHover = await readTokenRgb(page, "--primary-hover");

      // Same hue family as the branded primary — not the built-in magenta (~300°).
      expect(Math.abs(hue(primaryHover) - hue(primary))).toBeLessThan(12);

      // Still a distinct state: darker in light mode, brighter in dark mode.
      const delta = luminance(primaryHover) - luminance(primary);
      expect(mode === "light" ? delta : -delta).toBeLessThan(-4);
    });

    test(`--sidebar-ring aliases --ring in ${mode} mode`, async ({ page }) => {
      await page.addInitScript(
        (themeMode) => window.localStorage.setItem("launchpad:themeMode", themeMode),
        mode,
      );
      await page.goto("/");
      await expect(page.getByRole("button", { name: /account menu/i })).toBeVisible();

      expect(await readTokenRgb(page, "--sidebar-ring")).toEqual(
        await readTokenRgb(page, "--ring"),
      );
    });
  }
});
