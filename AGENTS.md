# AGENTS.md

This file provides guidance to coding agents when working with code in this repository.

## What this is

Nebari Landing ("the Launchpad") is the authenticated service-discovery portal for a Nebari Infrastructure Core (NIC) cluster. It is **two deployables in one repo**:

- **webapi** (Go, `cmd/` + `internal/`) — REST + WebSocket server. Watches `NebariApp` custom resources via the Kubernetes API, validates Keycloak JWTs, and stores pins / access-requests / notifications / WS tickets in Redis.
- **frontend** (`frontend/`) — Vite + React 19 SPA (shadcn/ui + Tailwind v4), served by nginx in production. Authenticates directly with Keycloak via keycloak-js (PKCE); **there is no server-side session**.

Both ship via the `charts/nebari-landing` Helm chart, normally reconciled by the Nebari Operator + ArgoCD. The Go binary and packaged chart are released together by GoReleaser.

## Architecture notes that span files

- **No proxy/session in front of the SPA.** The browser does PKCE against Keycloak, holds the JWT, and sends `Authorization: Bearer <jwt>` on REST calls. nginx only serves static files and proxies `/api/*` to the webapi.
- **WebSocket auth is a two-step exchange** because browsers can't set headers on a WS upgrade. The SPA `POST`s its Bearer JWT to get a single-use ticket (30s TTL, Redis-backed via `internal/wsticket`), then connects to `wss://.../ws?ticket=<id>`. Non-browser clients can send the Bearer token directly. See `docs/api.md#websocket-authentication`.
- **Two distinct Keycloak URLs.** `frontend.keycloak.url` is the public URL the browser redirects to. `webapi.keycloak.url` is the in-cluster URL the webapi uses to fetch JWKS; `webapi.keycloak.issuerUrl` is what the `iss` claim is validated against and must equal what the browser sees. Mismatches here are the usual cause of auth failures.
- **Service list is sourced from the cluster, not a database.** `internal/watcher` watches `NebariApp` CRs (controller-runtime) and feeds `internal/cache` (Redis). There is no manual service registration.
- **App wiring lives in `internal/app/app.go`** (`App` struct) — start there to see how watcher, cache, auth, the WS hub, and the API handlers are assembled. HTTP routes are all in `internal/api/handlers.go`.
- **Runtime branding/theming**: the frontend fetches `/config.json` (rendered from `values.yaml` by the chart) and applies title/logo/favicon/theme tokens *before React mounts* — no image rebuild to rebrand. Theme token values are sanitized against CSS-injection at runtime (see README "Branding & Theming").

## Common commands

### webapi (Go 1.25)
```sh
make build        # build bin/webapi
make test         # runs fmt + vet, then: go test ./internal/... -race -count=1 -coverprofile=coverage.out
make test-html    # HTML coverage report
make fmt / vet
make pf           # port-forward a deployed webapi to localhost:8080
```
Run a single Go test: `go test ./internal/api -run TestName -race -count=1 -v`.

E2E (Ginkgo, requires a live cluster with operator CRDs): `make test-e2e`.

### frontend (Node 22, npm — `cd frontend` first)
```sh
npm ci
npm run dev          # Vite at :5173 — does NOT proxy /api/*; needs a running webapi
npm run test         # vitest (watch);  npm run test:run for CI/one-shot
npm run check:fix    # biome check --write (lint + format; biome, not eslint/prettier)
npm run build        # tsc -b && vite build
npm run e2e          # playwright (chromium)
```

### Generated artifacts — regenerate, don't hand-edit
`make docs` regenerates everything downstream of the API handler annotations and OpenAPI spec, in order: OpenAPI 3.1 spec (`internal/api/docs/`, embedded at build time via swag annotations in `cmd/main.go` + `internal/api/handlers.go`) → SVG endpoint cards (`docs/assets/`) → `docs/api.md` → frontend MSW handlers (`frontend/src/mocks/generated/handlers.ts`). After editing any handler annotation, run `make docs` — the `openapi-drift` CI workflow fails if the committed artifacts are stale.

### Local dev cluster (minikube + Keycloak)
Driven by a **separate Makefile**: `make -f dev/Makefile <target>`. `setup` does the full bootstrap; `cluster-start` / `port-forward` / `dev-watch` / `uninstall` / `cluster-delete` manage the loop. `dev-watch` strategic-patches the deployed frontend pod into a Vite HMR server (minikube mount → live reload); **always `stop-dev-watch` before deploying a manually-built frontend image**, or the watcher overwrites it. Full walkthrough: `dev/QUICKSTART.md`. Lighter frontend-only loop (docker-compose + MSW, ~10s boot): `docs/dev-quickstart.md`.

## Conventions

- Frontend uses **Biome** for lint+format (not ESLint/Prettier) — `biome.json`. shadcn/ui components configured in `components.json`. API clients are one-file-per-endpoint under `frontend/src/api/`.
- `--enable-docs` (env `ENABLE_DOCS=true`, chart `webapi.docsEnabled=true`) exposes the OpenAPI spec + an HTML reference at `/api/v1/docs`. Both routes are absent without the flag — **never enable in production**.
- Releases are cut by the `release-prep` GitHub Actions workflow, not local make targets. See `docs/maintainers/release-checklist.md`.
