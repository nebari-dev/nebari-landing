import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/api/client", () => ({ apiFetch: vi.fn() }));
vi.mock("@/api/listServices", () => ({ listServices: vi.fn().mockResolvedValue([]) }));

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

  emitMessage(message: unknown): void {
    const event = new MessageEvent("message", { data: JSON.stringify(message) });
    for (const listener of this.listeners.message ?? []) listener(event);
  }
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
      }),
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
    mockedApiFetch.mockResolvedValueOnce(new Response("Unauthorized", { status: 401 }));

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

  it("ignores non-service WebSocket frames without updating hook state", async () => {
    mockedApiFetch.mockResolvedValueOnce(new Response("Unauthorized", { status: 401 }));
    let renderCount = 0;
    const user = { name: "alice" };

    const { result } = renderHook(() => {
      renderCount += 1;
      return useLaunchpadData(user);
    });

    await waitFor(() => expect(MockWebSocket.instances.length).toBe(1));
    await waitFor(() => expect(renderCount).toBeGreaterThan(1));
    const settledRenderCount = renderCount;

    act(() => {
      MockWebSocket.instances[0].emitMessage({
        type: "notification.created",
        notification: { id: "ignored" },
      });
    });

    expect(renderCount).toBe(settledRenderCount);
    expect(result.current).not.toHaveProperty("notifications");
  });
});
