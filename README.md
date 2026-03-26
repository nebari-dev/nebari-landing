<p align="center">
  <a href="https://nebari.dev">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/nebari-dev/nebari-design/main/logo-mark/horizontal/standard/Nebari-Logo-Horizontal-Lockup-White-text.png">
      <source media="(prefers-color-scheme: light)" srcset="https://raw.githubusercontent.com/nebari-dev/nebari-design/main/logo-mark/horizontal/standard/Nebari-Logo-Horizontal-Lockup.png">
      <img alt="Nebari" src="docs/Nebari-Logo-Horizontal-Lockup.png" width="300">
    </picture>
  </a>
</p>

<h1 align="center">Nebari Landing</h1>

<p align="center">
  <strong>The Launchpad for Nebari — service discovery and access portal.</strong><br /> A real-time, SSO-aware landing
  page that gives users one place to find and launch every service on the platform.
</p>

<p align="center">
  <img src="docs/static/imgs/landingpage-overview.png" alt="Nebari Landing page overview" width="800">
</p>

<p align="center">
  <a href="https://github.com/nebari-dev/nebari-landing/actions/workflows/webapi.yml"><img
  src="https://github.com/nebari-dev/nebari-landing/actions/workflows/webapi.yml/badge.svg" alt="WebAPI CI"></a> <a
  href="https://github.com/nebari-dev/nebari-landing/actions/workflows/frontend.yml"><img
  src="https://github.com/nebari-dev/nebari-landing/actions/workflows/frontend.yml/badge.svg" alt="Frontend CI"></a> <a
  href="https://github.com/nebari-dev/nebari-landing/blob/main/LICENSE"><img
  src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License"></a> <a
  href="https://github.com/nebari-dev/nebari-landing/releases/latest"><img
  src="https://img.shields.io/github/v/release/nebari-dev/nebari-landing?logo=github&label=release" alt="Latest
  Release"></a> <a href="https://golang.org"><img
  src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white" alt="Go 1.25+"></a> <a
  href="https://react.dev"><img src="https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black" alt="React
  19"></a>
</p>

<p align="center">
  <a href="#architecture">Architecture</a> &middot; <a href="#quick-start">Quick Start</a> &middot; <a
  href="#helm-install">Helm Install</a> &middot; <a href="#development">Development</a> &middot; <a
  href="docs/api.md">API Reference</a> &middot; <a href="dev/QUICKSTART.md">Local Dev Guide</a> &middot; <a
  href="CONTRIBUTING.md">Contributing</a>
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

Release artifacts — the Go webapi binary (linux/darwin, amd64/arm64) and the packaged Helm chart — are attached to every
[GitHub release](https://github.com/nebari-dev/nebari-landing/releases) via GoReleaser.

## Key Features

| Feature | Description |
| --- | --- |
| **Service Discovery** | Automatically surfaces every `NebariApp` on the cluster — no manual registration required |
| **Real-time Updates** | WebSocket hub pushes service changes and notifications to the browser instantly |
| **SSO-Aware** | OAuth2 Proxy + Keycloak JWT validation — users land authenticated, admins see admin controls |
| **Pins & Access Requests** | Users can pin favourite services and request access to restricted ones |
| **USWDS Design System** | Accessible, government-grade UI components out of the box |

## Helm Install

> **Note**: In a full Nebari / NIC deployment the chart is managed by the Nebari Operator and ArgoCD — you do not need
> to install it manually.

### Add the Helm repository

```sh
helm repo add nebari https://nebari-dev.github.io/helm-repository
helm repo update
```

### Install

```sh
helm upgrade --install nebari-landing nebari/nebari-landing \
  --namespace nebari-system --create-namespace \
  --set frontend.oauth2Proxy.oidcIssuerURL=https://<keycloak-host>/realms/<realm> \
  --set frontend.oauth2Proxy.clientID=<client-id> \
  --set frontend.oauth2Proxy.clientSecret=<client-secret> \
  --set frontend.oauth2Proxy.cookieSecret=<32-byte-base64-secret>
```

See [`charts/nebari-landing/values.yaml`](charts/nebari-landing/values.yaml) for the full set of configurable values.



## Quick Start

### Prerequisites

| Tool | Minimum version | Notes |
| --- | --- | --- |
| `docker` | 24+ | Used as the minikube driver |
| `kubectl` | 1.28+ | Cluster interaction |
| `helm` | 3.14+ | Installs Keycloak and PostgreSQL |
| `minikube` | any | Auto-downloaded to `.bin/` if absent |
| `node` | 22+ | Frontend development only (see `frontend/.node-version`) |
| `python3` | 3.10+ | Integration test script only |

See [dev/QUICKSTART.md](dev/QUICKSTART.md) for the full local dev walkthrough.

### Initial setup

```sh
# Setup the project
make -f dev/Makefile setup
```

### Start the local dev cluster

```sh
# Restart an existing cluster (after minikube was stopped or the host rebooted)
make -f dev/Makefile cluster-start
make -f dev/Makefile port-forward

# Tear down the deployed app (keeps the cluster running)
make -f dev/Makefile uninstall

# Or delete the cluster entirely
make -f dev/Makefile cluster-delete
```

### Front end development

```sh
# Start the dev-watch (hot reaload for the front end for continuous development)
make -f dev/Makefile dev-watch

# Stop dev-watch, - Run this when manually reloading the front end. (will result in errors if not)
make -f dev/Makefile stop-dev-watch
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

### Helm chart targets

```sh
# Package the chart into dist/
make helm-package

# Update Chart.yaml version and appVersion (does NOT commit values.yaml — CI pins image tags)
make helm-chart-version VERSION=0.2.0 APP_VERSION=v0.2.0

# Full release preparation (must be on a release tag)
make prepare-release
```

### Run the frontend in watch mode

```sh
cd frontend
npm ci
npm run dev
```

> **Note**: `npm run dev` serves the SPA at `http://localhost:5173` but does **not** proxy `/api/*` calls — those
> require a running webapi. For a fully connected local dev loop (Keycloak + webapi + frontend with hot-reload) use the
> dev cluster described in [dev/QUICKSTART.md](dev/QUICKSTART.md).

## Development

### Project structure

```
cmd/                  webapi entry point (main.go)
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
docs/
  ├── api.md          HTTP API reference (auto-generated)
  ├── design/         Architecture and design documents
  ├── maintainers/    Release checklist and maintainer guides
  └── static/imgs/    Screenshots and static assets
test/e2e/             End-to-end tests (Ginkgo)
tools/apidoc/         API documentation generator (go generate)
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

### Local GoReleaser snapshot

To test the full binary build locally before tagging:

```sh
goreleaser release --snapshot --clean
# Artifacts land in dist/
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

**Documentation**:
- **[Contributing Guide](CONTRIBUTING.md)** - Complete development workflow
- **[Release Checklist](docs/maintainers/release-checklist.md)** - For maintainers creating releases
- **[API Reference](docs/api.md)** - WebAPI endpoint documentation

See our [issue tracker](https://github.com/nebari-dev/nebari-landing/issues) for open issues.

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
