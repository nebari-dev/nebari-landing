// Stateful MSW handlers. These override a subset of the generated handlers
// so the SPA sees realistic mutation across calls (pin/unpin, mark-read,
// request access, approve/deny). Anything not listed here falls through to
// the generated layer in ./generated/handlers.ts.

import { HttpResponse, http } from "msw";
import { generatedHandlers } from "./generated/handlers";
import { store } from "./store";
import { wsHandlers } from "./ws";

const overrides = [
  http.get("/api/v1/health", () => HttpResponse.json({ status: "ok" })),

  http.get("/api/v1/caller-identity", () =>
    HttpResponse.json({
      authenticated: true,
      username: "dev",
      name: "Dev User",
      email: "dev@example.com",
      groups: ["/users", "/admin"],
    }),
  ),

  http.get("/api/v1/categories", () => HttpResponse.json(store.categories)),

  http.get("/api/v1/services", () => HttpResponse.json({ services: store.services })),

  http.get("/api/v1/services/:id", ({ params }) => {
    const service = store.services.find((s) => s.id === params.id);
    if (!service) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(service);
  }),

  http.post("/api/v1/services/:id/request_access", async ({ params, request }) => {
    const service = store.services.find((s) => s.id === params.id);
    if (!service) return new HttpResponse(null, { status: 404 });

    const body = (await request.json().catch(() => ({}))) as {
      userId?: string;
      message?: string;
    };

    const req = {
      id: `req-${store.accessRequests.length + 1}`,
      serviceUID: service.id,
      serviceName: service.name,
      userID: body.userId ?? "dev",
      userEmail: "dev@example.com",
      message: body.message ?? "",
      status: "pending" as const,
      requestedAt: new Date().toISOString(),
      resolvedAt: "",
      resolvedBy: "",
    };
    store.accessRequests.push(req);
    return HttpResponse.json({ success: true, message: "Request submitted" });
  }),

  http.get("/api/v1/notifications", () => HttpResponse.json(store.notifications)),

  http.put("/api/v1/notifications/:id/read", ({ params }) => {
    const n = store.notifications.find((x) => x.id === params.id);
    if (!n) return new HttpResponse(null, { status: 404 });
    n.read = true;
    return new HttpResponse(null, { status: 204 });
  }),

  http.get("/api/v1/pins", () => {
    const uids = store.services.filter((s) => s.pinned).map((s) => s.id);
    return HttpResponse.json({ pins: uids.map((id) => ({ id })), uids });
  }),

  http.get("/api/v1/pins/:uid", ({ params }) => {
    const svc = store.services.find((s) => s.id === params.uid);
    if (!svc?.pinned) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ id: svc.id });
  }),

  http.put("/api/v1/pins/:uid", ({ params }) => {
    const svc = store.services.find((s) => s.id === params.uid);
    if (!svc) return new HttpResponse(null, { status: 404 });
    svc.pinned = true;
    return new HttpResponse(null, { status: 204 });
  }),

  http.delete("/api/v1/pins/:uid", ({ params }) => {
    const svc = store.services.find((s) => s.id === params.uid);
    if (!svc) return new HttpResponse(null, { status: 404 });
    svc.pinned = false;
    return new HttpResponse(null, { status: 204 });
  }),

  // The webapi issues a short-lived single-use ticket the SPA puts on the WS
  // upgrade URL. Any opaque string works against MSW's ws interceptor.
  http.post("/api/v1/ws-ticket", () =>
    HttpResponse.json({ ticket: `mock-${Math.random().toString(36).slice(2, 10)}` }),
  ),

  http.get("/api/v1/admin/access-requests", ({ request }) => {
    const url = new URL(request.url);
    const status = url.searchParams.get("status");
    const list = status
      ? store.accessRequests.filter((r) => r.status === status)
      : store.accessRequests;
    return HttpResponse.json(list);
  }),

  http.put("/api/v1/admin/access-requests/:id/:action", ({ params }) => {
    const req = store.accessRequests.find((r) => r.id === params.id);
    if (!req) return new HttpResponse(null, { status: 404 });
    if (params.action !== "approve" && params.action !== "deny") {
      return new HttpResponse(null, { status: 400 });
    }
    req.status = params.action === "approve" ? "approved" : "denied";
    req.resolvedAt = new Date().toISOString();
    req.resolvedBy = "dev";
    return HttpResponse.json(req);
  }),
];

// MSW resolves handlers in order; overrides come first so they win over the
// generated fallbacks.
export const handlers = [...overrides, ...generatedHandlers, ...wsHandlers];
