#!/usr/bin/env bash
#
# test-webapi-locally.sh - Minimal test of webapi cache/visibility bug
#
# This script:
# 1. Deploys webapi with debug logging to local k3d cluster
# 2. Creates a public NebariApp
# 3. Queries the API to see if it appears
# 4. Shows debug logs

set -euo pipefail

echo "=== Testing webapi locally with debug logging ==="

# Check cluster
if ! kubectl get nodes &>/dev/null; then
    echo "Error: k3d cluster not accessible"
    exit 1
fi

# Clean up any previous test
echo "Cleaning up previous test resources..."
kubectl delete nebariapp test-visibility-app -n default --ignore-not-found=true
kubectl delete deployment,service webapi-test -n default --ignore-not-found=true
kubectl delete service test-backing-svc -n default --ignore-not-found=true

# Create backing service (required by NebariApp)
echo ""
echo "==> Creating backing service..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: test-backing-svc
  namespace: default
spec:
  selector:
    app: nonexistent  # Doesn't matter, just needs to exist
  ports:
  - port: 8080
    targetPort: 8080
EOF

# Deploy webapi with debug image
echo ""
echo "==> Deploying webapi with debug logging..."
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: webapi-test
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: webapi-test
  template:
    metadata:
      labels:
        app: webapi-test
    spec:
      containers:
      - name: webapi
        image: webapi-debug:local
        imagePullPolicy: Never
        ports:
        - containerPort: 8080
        env:
        - name: KEYCLOAK_ISSUER_URL
          value: "http://fake-keycloak"  # Not testing auth, just visibility
        - name: KEYCLOAK_REALM
          value: "nebari"
        - name: ENABLE_AUTH
          value: "false"  # Disable auth to simplify test
---
apiVersion: v1
kind: Service
metadata:
  name: webapi-test
  namespace: default
spec:
  selector:
    app: webapi-test
  ports:
  - port: 8080
    targetPort: 8080
EOF

echo "Waiting for webapi pod to be ready..."
kubectl wait --for=condition=ready pod -l app=webapi-test -n default --timeout=60s

# Create a public NebariApp
echo ""
echo "==> Creating public NebariApp..."
cat <<EOF | kubectl apply -f -
apiVersion: reconcilers.nebari.dev/v1
kind: NebariApp
metadata:
  name: test-visibility-app
  namespace: default
spec:
  hostname: test-app.local
  service:
    name: test-backing-svc
    port: 8080
  landingPage:
    enabled: true
    displayName: "Test Visibility App"
    description: "Testing public visibility"
    category: "Testing"
    priority: 100
  # NO auth block = should be public!
EOF

echo "Waiting 5 seconds for watcher to process..."
sleep 5

# Get webapi pod name
POD=$(kubectl get pod -l app=webapi-test -n default -o jsonpath='{.items[0].metadata.name}')

echo ""
echo "==> Checking webapi logs for [CACHE-DEBUG]..."
kubectl logs -n default "$POD" | grep -E "\[CACHE-DEBUG\]|\[API\]|Service added|Service updated" || echo "No debug logs found!"

echo ""
echo "==> Querying API for services..."
kubectl exec -n default "$POD" -- wget -q -O- http://localhost:8080/api/v1/services | jq '.'

echo ""
echo "=== Test complete! ==="
echo "Check the output above for:"
echo "1. [CACHE-DEBUG] logs showing visibility value"
echo "2. Whether 'Test Visibility App' appears in the API response"
