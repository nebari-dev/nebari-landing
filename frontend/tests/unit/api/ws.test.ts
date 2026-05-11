import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createWebSocketClient } from "@/api/ws";

// Minimal WebSocket stand-in that records its constructor URL and lets the
// test trigger a 'close' to exercise the auto-reconnect path.
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
  close(): void {
    this.dispatch("close");
  }

  dispatch(type: string, event: Event = new Event(type)): void {
    this.listeners[type]?.forEach((fn) => fn(event));
  }
}

beforeEach(() => {
  MockWebSocket.instances = [];
  vi.stubGlobal("WebSocket", MockWebSocket);
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("createWebSocketClient getQueryParams", () => {
  it("calls getQueryParams on connect and appends the result to the URL", async () => {
    const getQueryParams = vi.fn(async () => ({ ticket: "abc123" }));
    const client = createWebSocketClient({ path: "/ws", getQueryParams });

    client.connect();
    await vi.waitFor(() => expect(MockWebSocket.instances.length).toBe(1));

    expect(getQueryParams).toHaveBeenCalledTimes(1);
    const url = new URL(MockWebSocket.instances[0].url);
    expect(url.pathname).toBe("/api/v1/ws");
    expect(url.searchParams.get("ticket")).toBe("abc123");
  });

  it("re-invokes getQueryParams on auto-reconnect after the socket closes", async () => {
    let counter = 0;
    const getQueryParams = vi.fn(async () => ({ ticket: `t${++counter}` }));

    const client = createWebSocketClient({
      path: "/ws",
      getQueryParams,
      reconnectDelayMs: 50,
    });

    client.connect();
    await vi.waitFor(() => expect(MockWebSocket.instances.length).toBe(1));
    expect(getQueryParams).toHaveBeenCalledTimes(1);

    // Simulate the server closing the socket. The client schedules a reconnect.
    MockWebSocket.instances[0].dispatch("close");
    await vi.advanceTimersByTimeAsync(50);
    await vi.waitFor(() => expect(MockWebSocket.instances.length).toBe(2));

    expect(getQueryParams).toHaveBeenCalledTimes(2);
    const url2 = new URL(MockWebSocket.instances[1].url);
    expect(url2.searchParams.get("ticket")).toBe("t2");
  });

  it("does not reconnect after disconnect(), so getQueryParams is not called again", async () => {
    const getQueryParams = vi.fn(async () => ({ ticket: "once" }));
    const client = createWebSocketClient({
      path: "/ws",
      getQueryParams,
      reconnectDelayMs: 50,
    });

    client.connect();
    await vi.waitFor(() => expect(MockWebSocket.instances.length).toBe(1));

    client.disconnect();
    await vi.advanceTimersByTimeAsync(200);

    expect(getQueryParams).toHaveBeenCalledTimes(1);
    expect(MockWebSocket.instances.length).toBe(1);
  });

  it("falls back to static queryParams when getQueryParams is not provided", async () => {
    const client = createWebSocketClient({
      path: "/ws",
      queryParams: { token: "static" },
    });

    client.connect();
    await vi.waitFor(() => expect(MockWebSocket.instances.length).toBe(1));

    const url = new URL(MockWebSocket.instances[0].url);
    expect(url.searchParams.get("token")).toBe("static");
  });
});
