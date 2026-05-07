# E2E Test Refactoring: Remove kubectl Port-Forward

## Current Problem

**Test Failure:** test-pin-app is added to cache (confirmed in logs) but never appears in API response when queried through port-forward.

**Root Cause:** Using `exec.Command("kubectl", "port-forward", ...)` introduces:
- Race conditions (subprocess readiness)
- Port conflicts (parallel tests)
- Silent failures (no error visibility)
- Connection ambiguity (which pod/replica?)

## Proposed Solution: Direct Service Access

### Phase 1: Use In-Cluster Service DNS (Immediate Fix)

Since tests run in CI with cluster access, we can call services directly:

```go
// Before (lines 642-646, 658-667)
pfCmd = exec.Command("kubectl", "port-forward", "-n", namespace, "svc/webapi", "18084:8080")
pfCmd.Start()
webapiBase = "http://localhost:18084"
req, _ := http.NewRequest("GET", webapiBase+"/api/v1/services", nil)

// After
webapiBase = fmt.Sprintf("http://%s.%s.svc.cluster.local:8080", 
    e2eWebapiService, e2eNamespace)
req, _ := http.NewRequest("GET", webapiBase+"/api/v1/services", nil)
```

**Benefits:**
- ✅ No subprocess management
- ✅ No port conflicts
- ✅ Direct connection to actual service
- ✅ Kubernetes handles load balancing
- ✅ Same approach already used for Keycloak (line 624)!

### Phase 2: Create HTTP Client Helper (Better)

See `test/e2e/http_client.go` for a reusable client that:
- Auto-detects in-cluster vs out-of-cluster
- Handles authentication
- Provides clean API (Health(), GetServices(), etc.)
- Works for both CI and local development

```go
// Usage
client, err := NewWebAPIClient(config, namespace, serviceName)
client.BearerToken = token

// Health check
err = client.Health(ctx)

// Get services
req, _ := client.NewRequest("GET", "/api/v1/services")
resp, _ := client.Do(req)
```

### Phase 3: Add Better Error Logging (Debug)

When API queries fail, log:
- Request URL and headers (redacted token)
- Response status and body
- What services are in cache (from webapi logs)

This would have revealed the issue immediately.

## Implementation Plan

### Quick Win (1 hour)
Replace all `exec.Command("kubectl", "port-forward", ...)` with direct service URLs:

**Files to modify:**
1. `test/e2e/webapi_test.go` - Lines 642-646 (Pins context)
2. `test/e2e/webapi_test.go` - Similar patterns in other contexts
3. Remove `pfCmd` variables and cleanup code

**Changes:**
```diff
- By("Port-forwarding to webapi on :18084")
- pfCmd = exec.Command("kubectl", "port-forward",
-     "-n", e2eNamespace, fmt.Sprintf("svc/%s", e2eWebapiService), "18084:8080")
- Expect(pfCmd.Start()).NotTo(HaveOccurred())
- webapiBase = "http://localhost:18084"
+ By("Connecting directly to webapi service")
+ webapiBase = fmt.Sprintf("http://%s.%s.svc.cluster.local:8080",
+     e2eWebapiService, e2eNamespace)

- AfterAll(func() {
-     if pfCmd != nil && pfCmd.Process != nil {
-         _ = pfCmd.Process.Kill()
-     }
- })
+ // No cleanup needed
```

### Better Approach (2-3 hours)
Integrate the `WebAPIClient` helper:

1. Add `http_client.go` (already created)
2. Refactor tests to use client
3. Add helper methods for common operations:
   ```go
   func (c *WebAPIClient) GetServices(ctx context.Context) ([]Service, error)
   func (c *WebAPIClient) PinService(ctx context.Context, uid string) error
   func (c *WebAPIClient) GetPins(ctx context.Context) ([]Pin, error)
   ```

### Keycloak Port-Forward

The outer `BeforeAll` still uses port-forward for Keycloak (lines 287-306). This should also be replaced:

```go
// Instead of port-forwarding Keycloak, use service DNS
kcURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:8080", kcService, kcNamespace)
tokenReq, err := http.NewRequest(http.MethodPost,
    fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", kcURL, kcRealm),
    strings.NewReader(tokenForm.Encode()))
```

## Testing Strategy

1. **Local testing**: Use the local cluster setup
   ```bash
   ./scripts/setup-local-e2e.sh
   # Edit tests to use service DNS
   go test -v -tags=e2e ./test/e2e/
   ```

2. **CI validation**: Push and let CI run
   - Should fix the test-pin-app timeout issue
   - Should be faster (no subprocess overhead)
   - Should have fewer flakes

## Expected Outcome

**Before:**
- Tests timeout after 5 minutes
- Watcher has service, API doesn't return it
- Port conflicts between parallel contexts

**After:**
- Tests pass reliably
- Direct connection to service
- No subprocess management
- Clear error messages

## Next Steps

1. Implement Quick Win (direct service URLs)
2. Test locally with `./scripts/test-pins-scenario.sh`
3. Push to PR and validate in CI
4. If successful, refactor to use WebAPIClient helper
5. Document the approach for future tests
