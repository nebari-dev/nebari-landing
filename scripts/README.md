# E2E Testing Scripts

This directory contains scripts for local E2E testing and debugging.

## Scripts

### `setup-local-e2e.sh`
Sets up a local k3d cluster with the full Nebari platform for E2E testing.

**Usage:**
```bash
./scripts/setup-local-e2e.sh
```

**What it does:**
- Creates a k3d cluster (or reuses existing one)
- Installs NIC (Nebari Infrastructure CLI)
- Deploys the full platform (Keycloak, ArgoCD, MetalLB, nebari-operator, etc.)
- Builds and loads the webapi Docker image
- Deploys the webapi and frontend services

**Prerequisites:**
- Docker
- k3d
- kubectl
- jq
- curl

**Environment variables:**
- `CLUSTER_NAME` - k3d cluster name (default: `nebari-e2e-local`)
- `K8S_VERSION` - Kubernetes version (default: `v1.32.4-k3s1`)

### `debug-e2e.sh`
Collects debugging information from a running cluster.

**Usage:**
```bash
./scripts/debug-e2e.sh
```

**What it shows:**
- Webapi deployment status
- All NebariApp resources
- Webapi logs with cache operations
- Direct API endpoint test results
- Recent events

**Environment variables:**
- `NAMESPACE` - Kubernetes namespace (default: `nebari-system`)

### `test-pins-scenario.sh`
Reproduces the Pins BeforeAll test scenario that's failing in CI.

**Usage:**
```bash
./scripts/test-pins-scenario.sh
```

**What it does:**
1. Gets a Keycloak bearer token (like the test does)
2. Creates a test NebariApp (`test-pin-app`)
3. Polls the `/api/v1/services` API looking for the service
4. Reports success or failure
5. On failure, runs full diagnostics

**Environment variables:**
- `NAMESPACE` - Kubernetes namespace (default: `nebari-system`)
- `KEYCLOAK_PORT` - Local port for Keycloak port-forward (default: `19090`)
- `WEBAPI_PORT` - Local port for webapi port-forward (default: `18084`)
- `TIMEOUT` - Polling timeout in seconds (default: `60`)

## Typical Workflow

1. **Setup the cluster:**
   ```bash
   ./scripts/setup-local-e2e.sh
   ```

2. **Verify everything is running:**
   ```bash
   kubectl get pods -n nebari-system
   kubectl get pods -n keycloak
   ```

3. **Reproduce the test failure:**
   ```bash
   ./scripts/test-pins-scenario.sh
   ```

4. **If test fails, gather more info:**
   ```bash
   ./scripts/debug-e2e.sh
   ```

5. **Run the actual e2e tests:**
   ```bash
   # Get Keycloak admin password
   export NEBARI_REALM_ADMIN_PASSWORD=$(kubectl get secret -n keycloak keycloak-keycloakx-admin \
     -o jsonpath='{.data.password}' | base64 -d)
   
   # Run tests
   cd test/e2e
   ginkgo -v
   ```

6. **Clean up:**
   ```bash
   k3d cluster delete nebari-e2e-local
   ```

## Troubleshooting

### Cluster creation hangs
If k3d cluster creation hangs, try:
```bash
# Clean up any stale clusters
k3d cluster delete nebari-e2e-local

# Check Docker status
docker ps

# Try again
./scripts/setup-local-e2e.sh
```

### NIC deployment fails
Check the NIC logs and platform status:
```bash
# Check if platform is deployed
kubectl get applications -n argocd

# Check for errors
kubectl get events -A --field-selector type=Warning
```

### Webapi not responding
```bash
# Check deployment
kubectl get deployment -n nebari-system nebari-landing-webapi

# Check logs
kubectl logs -n nebari-system -l app.kubernetes.io/component=webapi

# Try restarting
kubectl rollout restart deployment/nebari-landing-webapi -n nebari-system
```

### Test-pin-app not appearing in API
This is the current CI failure we're debugging. Run:
```bash
./scripts/test-pins-scenario.sh
```

If it reproduces locally, you can:
- Examine webapi logs for watcher events
- Check if the NebariApp has a UID
- Verify the backing Service exists
- Test the API directly from inside the pod
- Add debug logging to the webapi code and rebuild

## CI Debugging

When tests fail in CI, the workflow automatically runs similar diagnostics.
Check the "Collect diagnostics on failure" step in the GitHub Actions logs.
