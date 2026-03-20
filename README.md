<p align="center">
  <a href="https://nebari.dev">
    <img src="https://raw.githubusercontent.com/nebari-dev/nebari/main/docs/_static/images/nebari-logo.svg" alt="Nebari" width="400">
  </a>
</p>

<h1 align="center">Nebari Landing</h1>

<p align="center">
  <strong>The Launchpad for Nebari — service discovery and access portal.</strong><br /> A real-time, SSO-aware landing
  page that gives users one place to find and launch every service on the platform.
</p>

<p align="center">
  <a href="https://github.com/nebari-dev/nebari-landing/actions/workflows/webapi.yml"><img
  src="https://github.com/nebari-dev/nebari-landing/actions/workflows/webapi.yml/badge.svg" alt="WebAPI CI"></a> <a
  href="https://github.com/nebari-dev/nebari-landing/actions/workflows/frontend.yml"><img
  src="https://github.com/nebari-dev/nebari-landing/actions/workflows/frontend.yml/badge.svg" alt="Frontend CI"></a> <a
  href="https://github.com/nebari-dev/nebari-landing/blob/main/LICENSE"><img
  src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License"></a> <a href="https://golang.org"><img
  src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white" alt="Go 1.25+"></a> <a
  href="https://react.dev"><img src="https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black" alt="React
  19"></a>
</p>

<p align="center">
  <a href="#architecture">Architecture</a> &middot; <a href="#quick-start">Quick Start</a> &middot; <a
  href="#development">Development</a> &middot; <a href="docs/api.md">API Reference</a> &middot; <a
  href="dev/QUICKSTART.md">Local Dev Guide</a>
</p>



> **Status**: Under active development as part of Nebari Infrastructure Core (NIC). APIs and behavior may change without
> notice.

## What is Nebari Landing?

Nebari Landing is the **Launchpad** — the entry point for every NIC-managed cluster. It surfaces all deployed
`NebariApp` services in a single, authenticated UI so users can discover and launch platform tools without knowing
individual URLs.

It is deployed automatically by the [Nebari Operator](https://github.com/nebari-dev/nebari-infrastructure-core) as part
of NIC's foundational software. Two components work together:

- **webapi** — Go REST API + WebSocket hub that watches `NebariApp` CRs via the Kubernetes API, validates JWTs from
  Keycloak, manages service pins, access requests, and real-time notifications over WebSocket.
- **frontend** — Vite/React SPA (USWDS design system) served by nginx and protected by an OAuth2 Proxy sidecar.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Browser                                                │
│    │                                                    │
│    ▼                                                    │
│  oauth2-proxy ──► Keycloak (JWT validation)             │
│    │                                                    │
│    ▼                                                    │
│  nginx (frontend SPA)                                   │
│    │  REST + WebSocket                                  │
│    ▼                                                    │
│  webapi                                                 │
│    ├── NebariApp CR watcher (Kubernetes API)            │
│    ├── Service cache + pins (Redis / in-memory)         │
│    ├── Access request store                             │
│    └── WebSocket hub (real-time notifications)          │
└─────────────────────────────────────────────────────────┘
```

Both pods are deployed via the `charts/nebari-landing` Helm chart, typically managed by ArgoCD through NIC.

## Key Features

| Feature | Description |
| --- | --- |
| **Service Discovery** | Automatically surfaces every `NebariApp` on the cluster — no manual registration required |
| **Real-time Updates** | WebSocket hub pushes service changes and notifications to the browser instantly |
| **SSO-Aware** | OAuth2 Proxy + Keycloak JWT validation — users land authenticated, admins see admin controls |
| **Pins & Access Requests** | Users can pin favourite services and request access to restricted ones |
| **USWDS Design System** | Accessible, government-grade UI components out of the box |

## Quick Start

### Prerequisites

| Tool | Minimum version | Notes |
| --- | --- | --- |
| `docker` | 24+ | Used as the minikube driver |
| `kubectl` | 1.28+ | Cluster interaction |
| `helm` | 3.14+ | Installs Keycloak and PostgreSQL |
| `minikube` | any | Auto-downloaded to `.bin/` if absent |
| `python3` | 3.10+ | Integration test script only |

See [dev/QUICKSTART.md](dev/QUICKSTART.md) for the full local dev walkthrough.

### Initial setup

```sh
# Setup the project
make -f dev/Makefile setup
```

### Start the local dev cluster

```sh
# Bring up minikube + Keycloak + the landing page stack
make -f dev/Makefile up

# Tear everything down
make -f dev/Makefile down
```

### Front end development

```sh
# Start the dev-watch (hot reaload for the front end for continuous development)
make -f dev/Makefile dev-watch

# Stop dev-watch, - Run this when manually reloading the front end. (will result in errors if not)
make -f dev/Makefile stop-dev-watch
```

### Build and run the webapi

```sh
# Build the binary
make build

# Run unit tests
make test

# Port-forward a running webapi deployment
make pf
```


## Development

### Project structure

```
cmd/                  webapi entry point
internal/
  ├── accessrequests/ Access request store
  ├── api/            HTTP handlers and routes
  ├── app/            Application wiring
  ├── auth/           JWT validation
  ├── cache/          Service cache (backed by Redis)
  ├── health/         Health check endpoint
  ├── keycloak/       Keycloak client
  ├── notifications/  Notification store
  ├── pins/           Pin store
  ├── watcher/        NebariApp CR watcher
  └── websocket/      WebSocket hub
frontend/
  src/
    ├── api/          Typed API clients
    ├── app/          App shell and SCSS
    ├── auth/         Keycloak integration
    └── components/   UI components
charts/nebari-landing/ Helm chart
dev/                  Local dev environment (minikube + Keycloak)
test/e2e/             End-to-end tests (Ginkgo)
```

### Running tests

```sh
# Unit tests with coverage
make test

# HTML coverage report
make test-html

# End-to-end tests (requires a live cluster with CRDs installed)
make test-e2e
```

### API Reference

See [docs/api.md](docs/api.md) for the full HTTP API reference.

To regenerate after editing route definitions:

```sh
go generate ./internal/api/...
```

### Code quality

```sh
make fmt   # go fmt
make vet   # go vet
```

## Contributing

Contributions are welcome! To get started:

```sh
git clone https://github.com/nebari-dev/nebari-landing.git
cd nebari-landing

# Build the webapi binary
make build

# Run unit tests
make test

# Start the local dev environment
make -f dev/Makefile up
```

See our [issue tracker](https://github.com/nebari-dev/nebari-landing/issues) for open issues.

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
