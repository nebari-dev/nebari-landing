#!/usr/bin/env bash
# Simple local reproduction script for the visibility bug
# This doesn't need a full cluster - it's for analyzing the logic

set -euo pipefail

echo "==================================================================="
echo " Analyzing Service Visibility Bug"
echo "==================================================================="
echo ""

echo "## Issue Summary"
echo "- Watcher logs show: 'Service added' for test-pin-app"
echo "- Cache operations succeed (health-checker shows service in cache)"
echo "- API /api/v1/services endpoint returns empty list"
echo "- Test timeout after 300 seconds"
echo ""

echo "## Key Code Paths"
echo ""
echo "1. Watcher (onAdd) -> Cache.Add()"
echo "   Location: internal/watcher/watcher.go:219-256"
echo "   - Converts NebariApp to App domain object"
echo "   - Calls cache.Add()"
echo ""

echo "2. Cache.Add() stores ServiceInfo"
echo "   Location: internal/cache/service_cache.go:60-100"
echo "   - Extracts visibility (defaults to 'private')"
echo "   - Stores in map[uid]*ServiceInfo"
echo ""

echo "3. API Handler (handleGetServices)"
echo "   Location: internal/api/handlers.go:320-342"
echo "   - Calls cache.GetAll()"
echo "   - Filters by canAccessService()"
echo ""

echo "4. Access Check (canAccessService)"
echo "   Location: internal/api/handlers.go:971-989"
echo "   - visibility='public' -> return true"
echo "   - visibility='private' -> requires authentication"
echo ""

echo "## Test NebariApp Spec"
echo "From test/e2e/webapi_test.go:186-225 (newNebariApp):"
echo ""
cat <<'YAML'
spec:
  landingPage:
    enabled: true
    displayName: "Test Service {name}"
    visibility: "public"    # <-- Should make service visible!
    priority: 100
YAML

echo ""
echo "## Hypothesis"
echo ""
echo "The test creates a NebariApp with visibility='public', which should make"
echo "it visible to unauthenticated users. But the API isn't returning it."
echo ""
echo "Possible causes:"
echo "1. UID not being set correctly (cache keyed by UID)"
echo "2. Timing - API called before cache.Add() completes"
echo "3. Multiple webapi pods - test hitting different pod than watcher"
echo "4. Cache corruption - service added then removed"
echo "5. Test creating NebariApp but not backing Service"
echo ""

echo "## Debug Logging Added (commit d170fb9)"
echo ""
echo "handlers.go: handleGetServices()"
echo "  - Total services in cache"
echo "  - Per-service access check with visibility"
echo "  - Final count returned"
echo ""
echo "service_cache.go: Add() and GetAll()"
echo "  - Service details when added"
echo "  - List of all services on retrieval"
echo ""

echo "## Next CI Run Will Show"
echo ""
echo "Expected logs (if working):"
echo "  [CACHE-DEBUG] Service added: name=test-pin-app visibility=public"
echo "  [API] handleGetServices called: totalServicesInCache=1"
echo "  [API] Service access check: serviceName=test-pin-app visibility=public canAccess=true"
echo "  [API] Returning services: count=1"
echo ""
echo "If test still fails, logs will show the exact filtering/visibility issue."
echo ""

echo "==================================================================="
echo " Monitoring CI"
echo "==================================================================="

gh run list --repo nebari-dev/nebari-landing --branch test/expand-e2e-unit-coverage-60 --limit 1 | head -3
