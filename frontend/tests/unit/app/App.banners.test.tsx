import { renderWithProviders as render } from "@/test/render";
import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/auth/keycloak", () => ({ signOut: vi.fn() }));
vi.mock("@/auth/user", () => ({ useUser: () => ({ user: null }) }));
vi.mock("@/hooks/useLaunchpadData", () => ({
  useLaunchpadData: () => ({
    services: [],
    onTogglePin: vi.fn(),
  }),
}));
vi.mock("@/app/config", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/app/config")>()),
  getAppConfig: vi.fn(),
}));

import { getAppConfig } from "@/app/config";
import App from "@/app/index";

const mockedGetAppConfig = vi.mocked(getAppConfig);

const baseConfig = {
  keycloak: { url: "http://localhost", realm: "nebari", clientId: "spa" },
};

beforeEach(() => {
  mockedGetAppConfig.mockReturnValue(baseConfig);
});

describe("App banners", () => {
  it("renders a top banner above the header and a bottom banner below the content", () => {
    mockedGetAppConfig.mockReturnValue({
      ...baseConfig,
      banners: { top: { text: "CUI TOP" }, bottom: { text: "CUI BOTTOM" } },
    });

    render(<App />);

    const top = screen.getByText("CUI TOP");
    const bottom = screen.getByText("CUI BOTTOM");
    const header = screen.getByRole("banner");

    expect(top.compareDocumentPosition(header) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(header.compareDocumentPosition(bottom) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("renders no banners when none are configured", () => {
    render(<App />);
    expect(screen.queryByRole("note")).toBeNull();
  });
});
