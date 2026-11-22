import { renderWithProviders as render } from "@/test/render";
import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { User } from "@/auth/user";

const state: { user: User | null } = { user: null };

vi.mock("@/auth/keycloak", () => ({ signOut: vi.fn() }));
vi.mock("@/auth/user", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/auth/user")>()),
  useUser: () => ({ user: state.user, loading: false }),
}));
vi.mock("@/hooks/useLaunchpadData", () => ({
  useLaunchpadData: () => ({ services: [], status: "ready", onTogglePin: vi.fn() }),
}));
vi.mock("@/app/config", () => ({ getAppConfig: () => null }));

import App from "@/app/index";

describe("App greeting", () => {
  beforeEach(() => {
    state.user = null;
  });

  it("greets the user by first name", () => {
    state.user = { name: "Samantha Lee", email: "sam@example.com", firstName: "Sam" };
    render(<App />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(/^Welcome back, Sam$/);
  });

  it("does not greet by username when the user has no name", () => {
    state.user = { name: "jdoe@corp.com", email: "jdoe@corp.com" };
    render(<App />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(/^Welcome back$/);
  });
});
