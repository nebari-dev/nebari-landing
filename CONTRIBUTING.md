# Contributing to Nebari Landing

Thank you for your interest in contributing! This guide will help you get started with development.

## Development Workflow

### Prerequisites

- Go 1.23 or later
- Node.js 22+ (see `frontend/.node-version`)
- Docker or Podman (for building images)
- Kubernetes cluster (kind, k3d, minikube, or cloud provider)
- `kubectl` configured to access your cluster
- Nebari Operator installed in your cluster (for NebariApp CRD)

### Setup

1. **Clone the repository**
   ```bash
   git clone https://github.com/nebari-dev/nebari-landing.git
   cd nebari-landing
   ```

2. **Install dependencies**
   ```bash
   go mod download
   cd frontend && npm ci && cd ..
   ```

3. **Verify your setup**
   ```bash
   make test  # Run backend unit tests
   cd frontend && npm run lint && npm run build && cd ..  # Verify frontend
   ```

## Making Changes

### Modifying the Backend (webapi)

When adding or changing backend functionality:

1. **Make your changes** to Go code in `internal/` or `cmd/`
2. **Run tests and linting**:
   ```bash
   make fmt vet test
   ```

3. **Test locally** (see Local Development section below)

4. **Commit**:
   ```bash
   git add internal/ cmd/
   git commit -m "feat: add new endpoint"
   ```

### Modifying the Frontend

When changing the React UI:

1. **Make your changes** in `frontend/src/`
2. **Run linting and build**:
   ```bash
   cd frontend
   npm run lint
   npm run build
   cd ..
   ```

3. **Test locally** (see Local Development section below)

4. **Commit**:
   ```bash
   git add frontend/
   git commit -m "feat: improve service card UI"
   ```

### Modifying the Helm Chart

When updating deployment configuration:

1. **Make your changes** in `charts/nebari-landing/`
2. **Test locally**:
   ```bash
   helm template charts/nebari-landing/ | kubectl apply --dry-run=client -f -
   ```

3. **Commit**:
   ```bash
   git add charts/
   git commit -m "fix: update resource limits"
   ```

**Note**: Chart versioning is handled during releases. Don't manually update `Chart.yaml` versions.

## Local Development

### Running Locally with Docker Compose

The fastest way to develop is using the local dev environment:

```bash
cd dev
make setup    # Start Kind cluster with operator
make up       # Start webapi + frontend + Redis
```

See [dev/QUICKSTART.md](dev/QUICKSTART.md) for detailed instructions.

### Running Backend Only

To run just the webapi against your cluster:

```bash
# Ensure you have NebariApp CRDs and a service account
make deploy

# Or run locally (watches your cluster)
go run ./cmd/main.go
```

### Running Frontend in Development Mode

```bash
cd frontend
npm run dev  # Starts Vite dev server on http://localhost:5173
```

**Note**: The frontend expects the webapi at `http://localhost:8080` by default.

### Testing in a Kind Cluster

Build and deploy both components:

```bash
make dev  # Builds image, loads to Kind, deploys to nebari-system
```

Port-forward to access:

```bash
make pf  # Forwards webapi to localhost:8080
```

## Pull Request Process

1. **Create a feature branch**:
   ```bash
   git checkout -b feat/my-feature
   ```

2. **Make your changes** following the guidelines above

3. **Ensure tests pass**:
   ```bash
   make test
   cd frontend && npm run lint && npm run build && cd ..
   ```

4. **Push and create PR**:
   ```bash
   git push origin feat/my-feature
   ```
   Then open a PR on GitHub.

5. **CI Checks**: Your PR must pass:
   - ✅ Backend tests and linting (Go)
   - ✅ Frontend linting and build (ESLint, Vite)
   - ✅ Docker image builds successfully

## Common Tasks

### Adding a New WebAPI Endpoint

1. Add handler in `internal/handlers/`
2. Register route in `internal/server/server.go`
3. Add tests in `internal/handlers/*_test.go`
4. Update API documentation in `docs/api.md`

### Adding a New Frontend Component

1. Create component in `frontend/src/components/`
2. Follow USWDS design system patterns
3. Ensure TypeScript types are correct
4. Test responsiveness

### Updating Dependencies

**Backend**:
```bash
go get -u ./...
go mod tidy
```

**Frontend**:
```bash
cd frontend
npm update
npm audit fix
cd ..
```

## Debugging

**Enable debug logging** for webapi:
```bash
kubectl set env deployment/webapi \
  LOG_LEVEL=debug \
  -n nebari-system
```

**Check webapi logs**:
```bash
kubectl logs -f deployment/webapi -c api -n nebari-system
```

**Frontend dev tools**: Use browser DevTools → Network tab to inspect API calls.

## Code Style

- **Go**: Run `make fmt vet` before committing
- **TypeScript/React**: Use `npm run lint` to check style
- Follow existing patterns in the codebase
- Write clear commit messages (conventional commits preferred)

## Release Process

Releases are automated. See [docs/maintainers/release-checklist.md](docs/maintainers/release-checklist.md) for maintainer instructions.

## Getting Help

- **Documentation**: See [README.md](README.md) and [docs/](docs/)
- **Local dev guide**: [dev/QUICKSTART.md](dev/QUICKSTART.md)
- **API reference**: [docs/api.md](docs/api.md)
- **Issues**: Open an issue on GitHub for bugs or feature requests
