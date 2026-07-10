---
title: Nebari Landing
description: The Launchpad for Nebari — service discovery and access portal, deployed via the nebari-landing Helm chart and reconciled by the Nebari Operator.
---

Nebari Landing is the **Launchpad** for a Nebari Infrastructure Core (NIC) cluster. It surfaces every deployed `NebariApp` service in a single, authenticated UI so users can discover and launch platform tools without knowing individual URLs.

## What ships in this repo

- **webapi** — Go REST API + WebSocket hub. Watches `NebariApp` custom resources via the Kubernetes API, validates JWTs from Keycloak, manages service pins, access requests, and real-time notifications.
- **frontend** — Vite + React SPA (shadcn/ui + Tailwind), served by nginx in production. Authenticates directly with Keycloak via `keycloak-js` (PKCE) — no server-side session, no reverse-auth proxy.
- **charts/nebari-landing** — the Helm chart both are deployed from.

Both components are released together and normally reconciled by ArgoCD through NIC's operator layer.

## Getting started

- Local frontend-only dev loop (docker-compose + MSW): [Frontend Dev Quickstart](/dev-quickstart/).
- Full local cluster via minikube + Keycloak: `dev/QUICKSTART.md` in the repo.
- API reference: `docs/api.md` (regenerated from swag annotations).

## Where things live

| Concern | Location |
| --- | --- |
| HTTP handlers and routes | `internal/api/handlers.go` |
| WebSocket hub and access policy | `internal/websocket/` |
| Watcher for `NebariApp` CRs | `internal/watcher/` |
| Redis-backed stores (pins, notifications, access requests, WS tickets) | `internal/{pins,notifications,accessrequests,wsticket}/` |
| Helm chart | `charts/nebari-landing/` |
| Local dev tooling | `dev/` |

## Contributing

See `CONTRIBUTING.md` for the branch and PR conventions used across the Nebari ecosystem. Design decisions live under `docs/design/` in the repo; when a change is large enough to justify prose, add a short ADR-style note there and link it from the PR body.
