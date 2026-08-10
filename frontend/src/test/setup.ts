import "@testing-library/jest-dom/vitest";
import { afterAll, afterEach, beforeAll } from "vitest";
import { server } from "../mocks/server";
import { resetStore } from "../mocks/store";

// jsdom does not implement ResizeObserver, which the Banner uses to publish
// its rendered height.
if (typeof globalThis.ResizeObserver === "undefined") {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

// Existing tests that mock @/api/client directly never reach fetch, so the
// MSW server is effectively dormant for them. `onUnhandledRequest: "bypass"`
// keeps such tests passing if they ever do call fetch for an unrelated URL.
beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));

afterEach(() => {
  server.resetHandlers();
  resetStore();
});

afterAll(() => server.close());
