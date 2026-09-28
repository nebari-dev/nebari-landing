import { renderWithProviders as render } from "@/test/render";
import { screen, waitFor } from "@testing-library/react";
import { HttpResponse, http } from "msw";
import { describe, expect, it, vi } from "vitest";
import {
  isAdminIdentity,
  normalizeGroupName,
  useCallerIdentity,
} from "@/hooks/useCallerIdentity";
import { server } from "@/mocks/server";

vi.mock("@/auth/keycloak", () => ({
  getToken: async () => "mock-token",
}));

function Probe({ signedIn }: { signedIn: boolean }) {
  const { isAdmin, isLoading } = useCallerIdentity(
    signedIn ? { name: "Dev User", email: "dev@example.com" } : null,
  );
  return <p>{isLoading ? "loading" : isAdmin ? "admin" : "not-admin"}</p>;
}

describe("normalizeGroupName / isAdminIdentity", () => {
  it("strips Keycloak path slashes", () => {
    expect(normalizeGroupName("/admin")).toBe("admin");
    expect(normalizeGroupName("admin")).toBe("admin");
  });

  it("requires authentication and admin-group membership", () => {
    expect(isAdminIdentity(undefined)).toBe(false);
    expect(isAdminIdentity({ authenticated: false, groups: ["/admin"] })).toBe(false);
    expect(isAdminIdentity({ authenticated: true, groups: ["/users"] })).toBe(false);
    expect(isAdminIdentity({ authenticated: true, groups: ["/users", "/admin"] })).toBe(true);
    expect(isAdminIdentity({ authenticated: true, groups: ["ops"] }, "ops")).toBe(true);
  });
});

describe("useCallerIdentity", () => {
  it("does not fetch for anonymous visitors", () => {
    render(<Probe signedIn={false} />);
    expect(screen.getByText("not-admin")).toBeInTheDocument();
  });

  it("reports admin from the server-trusted identity", async () => {
    render(<Probe signedIn />);
    await waitFor(() => expect(screen.getByText("admin")).toBeInTheDocument());
  });

  it("reports non-admin when the groups claim lacks the admin group", async () => {
    server.use(
      http.get("/api/v1/caller-identity", () =>
        HttpResponse.json({ authenticated: true, username: "dev", groups: ["/users"] }),
      ),
    );
    render(<Probe signedIn />);
    await waitFor(() => expect(screen.getByText("not-admin")).toBeInTheDocument());
  });
});
