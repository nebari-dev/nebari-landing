import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const kc: { authenticated: boolean; idTokenParsed?: Record<string, string> } = {
  authenticated: true,
};
vi.mock("@/auth/keycloak", () => ({ getKeycloakInstance: () => kc }));

import { useUser } from "@/auth/user";

describe("useUser", () => {
  beforeEach(() => {
    kc.authenticated = true;
  });

  it("uses given_name for firstName", () => {
    kc.idTokenParsed = { name: "Samantha Lee", given_name: "Sam", email: "sam@example.com" };
    const { result } = renderHook(() => useUser());
    expect(result.current.user).toEqual({
      name: "Samantha Lee",
      email: "sam@example.com",
      firstName: "Sam",
    });
  });

  it("falls back to the first word of name", () => {
    kc.idTokenParsed = { name: "Samantha Lee" };
    const { result } = renderHook(() => useUser());
    expect(result.current.user?.firstName).toBe("Samantha");
  });

  it("leaves firstName undefined when only preferred_username is set", () => {
    kc.idTokenParsed = { preferred_username: "jdoe@corp.com", sub: "abc" };
    const { result } = renderHook(() => useUser());
    expect(result.current.user?.name).toBe("jdoe@corp.com");
    expect(result.current.user?.firstName).toBeUndefined();
  });

  it("returns null when not authenticated", () => {
    kc.authenticated = false;
    const { result } = renderHook(() => useUser());
    expect(result.current.user).toBeNull();
  });
});
