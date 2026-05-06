#!/bin/bash
set -e

# Debug script for e2e test failures
# Run this after setup-local-e2e.sh to diagnose issues

NAMESPACE="${NAMESPACE:-nebari-system}"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "E2E Debugging Information"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo ""
echo "→ Webapi Deployment Status"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
kubectl get deployment -n "$NAMESPACE" nebari-landing-webapi -o wide || true
echo ""
kubectl describe deployment -n "$NAMESPACE" nebari-landing-webapi | tail -20 || true

echo ""
echo "→ Webapi Pods"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=webapi -o wide || true

echo ""
echo "→ NebariApp Resources"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
kubectl get nebariapps -n "$NAMESPACE" -o wide || true

echo ""
echo "→ NebariApp Details"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
for app in $(kubectl get nebariapps -n "$NAMESPACE" -o name 2>/dev/null); do
    echo ""
    echo "--- $app ---"
    kubectl get "$app" -n "$NAMESPACE" -o yaml
done

echo ""
echo "→ Test Service (backing service for NebariApps)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
kubectl get service test-service -n "$NAMESPACE" -o yaml 2>/dev/null || echo "✗ test-service not found"

echo ""
echo "→ Webapi Logs - Cache Operations"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
kubectl logs -n "$NAMESPACE" \
    -l app.kubernetes.io/component=webapi --tail=500 | \
    grep -E "(Service added|Service updated|Service deleted|Service removed|total:|Reconcile|Redis connected|Cache synced)" || \
    echo "No cache operation logs found"

echo ""
echo "→ Test API Endpoint Directly"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=webapi -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$POD" ]; then
    echo "Testing from pod: $POD"
    echo ""
    echo "Health check:"
    kubectl exec -n "$NAMESPACE" "$POD" -- curl -s http://localhost:8080/api/v1/health | jq '.' || true
    echo ""
    echo "Services list:"
    kubectl exec -n "$NAMESPACE" "$POD" -- curl -s http://localhost:8080/api/v1/services | jq '.' || true
else
    echo "✗ No webapi pod found"
fi

echo ""
echo "→ Recent Events"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
kubectl get events -n "$NAMESPACE" --sort-by='.lastTimestamp' | tail -20 || true

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Debugging complete"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
