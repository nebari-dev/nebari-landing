import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";

vi.mock("@/api/client", () => ({ apiFetch: vi.fn() }));
vi.mock("@/api/listServices", () => ({ listServices: vi.fn().mockResolvedValue([]) }));
vi.mock("@/api/notifications", () => ({
  listNotifications: vi.fn().mockResolvedValue([]),
  markNotificationRead: vi.fn().mockResolvedValue(undefined),
}));

import { apiFetch } from "@/api/client";
import { useLaunchpadData } from "@/hooks/useLaunchpadData";

class MockWebSocket {
  static instances: MockWebSocket[] = [];
  readonly url: string;
  readyState = 0;
  private listeners: Record<string, Array<(event: Event) => void>> = {};

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }

  addEventListener(type: string, fn: (event: Event) => void): void {
    (this.listeners[type] ??= []).push(fn);
  }

  removeEventListener(): void {}
  send(): void {}
  close(): void {}
}

const mockedApiFetch = vi.mocked(apiFetch);

beforeEach(() => {
  MockWebSocket.instances = [];
  vi.stubGlobal("WebSocket", MockWebSocket);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("useLaunchpadData ws-ticket exchange", () => {
  it("POSTs to /ws-ticket before opening the WS and embeds the ticket in the URL", async () => {
    mockedApiFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ ticket: "hex-ticket-1" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    );

    renderHook(() => useLaunchpadData({ name: "alice" }));

    await waitFor(() => {
      expect(mockedApiFetch).toHaveBeenCalledWith("/ws-ticket", { method: "POST" });
    });
    await waitFor(() => expect(MockWebSocket.instances.length).toBe(1));

    const url = new URL(MockWebSocket.instances[0].url);
    expect(url.pathname).toBe("/api/v1/ws");
    expect(url.searchParams.get("ticket")).toBe("hex-ticket-1");
  });

  it("connects without a ticket when /ws-ticket returns non-OK", async () => {
    mockedApiFetch.mockResolvedValueOnce(
      new Response("Unauthorized", { status: 401 })
    );

    renderHook(() => useLaunchpadData(null));

    await waitFor(() => expect(MockWebSocket.instances.length).toBe(1));
    expect(MockWebSocket.instances[0].url).not.toContain("ticket=");
  });

  it("connects without a ticket when /ws-ticket throws (auth-disabled dev)", async () => {
    mockedApiFetch.mockRejectedValueOnce(new Error("network down"));

    renderHook(() => useLaunchpadData(null));

    await waitFor(() => expect(MockWebSocket.instances.length).toBe(1));
    expect(MockWebSocket.instances[0].url).not.toContain("ticket=");
  });
});
