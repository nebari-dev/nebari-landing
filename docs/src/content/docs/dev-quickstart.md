---
title: Frontend Dev Quickstart
description: docker-compose + MSW loop for iterating on the SPA without a real webapi, operator, or CRD watcher.
---

This is the recommended path when you are iterating on the SPA and do **not** need a real webapi, operator, or CRD watcher. It boots in seconds, gives you a real Keycloak login (PKCE), and serves every `/api/v1/*` request from in-memory MSW mocks driven by the same OpenAPI spec the real webapi ships.

For workflows that *do* require the operator, oauth2-proxy, or live CRDs, use the full minikube path in [`dev/QUICKSTART.md`](https://github.com/nebari-dev/nebari-landing/blob/main/dev/QUICKSTART.md). The two paths are independent and can coexist.

## Prerequisites

| Tool | Minimum |
| --- | --- |
| `docker` | 24+ (with `docker compose`) |
| `node` | 22+ - see `frontend/.node-version` |

## One-time setup

```sh
# Install JS deps
npm --prefix frontend ci

# Bootstrap the local env file if you don't have one already.
# `frontend/.env` is gitignored - your local toggles live there.
cp frontend/.env.example frontend/.env

# Open frontend/.env and set VITE_USE_MOCKS=1.
```

## Every session

```sh
# 1. Start Keycloak + Postgres
docker compose -f dev/compose/docker-compose.yml up -d

# 2. Wait for Keycloak to import the realm (~10-20s on first boot)
curl -fsS http://localhost:8180/realms/nebari/.well-known/openid-configuration

# 3. Start the SPA
npm --prefix frontend run dev
```

Open `http://localhost:5173`. You'll be redirected to Keycloak at `http://localhost:8180`. Log in with one of the seeded users:

| Username | Password | Groups |
| --- | --- | --- |
| `admin` | `password` | `/admin`, `/users` |
| `dev` | `password` | `/users` |

After login you'll land on the launchpad with mocked services, notifications, and a live `/ws` connection emitting demo events.

## Shutting down

```sh
docker compose -f dev/compose/docker-compose.yml down       # keep volume
docker compose -f dev/compose/docker-compose.yml down -v    # wipe Postgres / realm
```

The MSW worker is registered against `localhost:5173`. Stopping Vite is enough to stop intercepting requests.

## Working with the mocks

The mock layer lives under `frontend/src/mocks/`:

- `fixtures.ts` - initial seed data (edit freely)
- `store.ts` - in-memory state shared across requests
- `handlers.ts` - stateful overrides (pin/unpin, mark-read, ...)
- `ws.ts` - WebSocket interceptor for `/api/v1/ws`
- `browser.ts` - `setupWorker` entry point
- `server.ts` - `setupServer` entry point (Vitest)
- `generated/handlers.ts` - auto-generated from `internal/api/docs/swagger.json`

To add fresh mock data, edit `fixtures.ts`. To change how an endpoint responds, add an `http.<verb>(...)` to the `overrides` array in `handlers.ts` - overrides take precedence over the generated fallback.

### When the OpenAPI spec changes

The generated handlers are committed to git and validated by the `openapi-drift` workflow. Regenerate them whenever you touch a Go handler annotation:

```sh
make docs
```

That single target rebuilds `swagger.json`, the SVG cards, `docs/api.md`, **and** `frontend/src/mocks/generated/handlers.ts`. Commit everything it touches.

### Testing

Vitest already wires the MSW Node server into `src/test/setup.ts`. Component tests that touch `apiFetch` will be served by the same handlers used in the browser. The store is reset between tests via `afterEach`.

```sh
npm --prefix frontend run test:run
```

## Disabling mocks

Set `VITE_USE_MOCKS=0` (or remove the line) in `frontend/.env`. Vite will then proxy `/api/*` and WebSocket upgrades to `WEBAPI_URL`, defaulting to `http://localhost:8080`. The minikube `dev-watch` path keeps its in-cluster default when `VITE_USE_POLLING=true`.

## Troubleshooting

**Login loop / `invalid_redirect_uri`.** The seeded `nebari-frontend-spa` client whitelists `http://localhost:*`. If you serve the SPA on a different host, add it under the client's *Valid Redirect URIs* in the Keycloak admin console (`http://localhost:8180`, admin/admin).

**"Worker script not registered."** The browser caches service workers. Hard reload (Cmd-Shift-R) after toggling `VITE_USE_MOCKS`.

**Stale realm state.** `docker compose down -v` wipes the Postgres volume so the next `up` re-imports `dev/keycloak/nebari-realm.json` from scratch.
