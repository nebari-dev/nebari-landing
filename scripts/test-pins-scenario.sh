#!/bin/bash
set -e

# Reproduce the Pins BeforeAll failure locally
# This creates a test NebariApp and polls the API like the e2e test does

NAMESPACE="${NAMESPACE:-nebari-system}"
KEYCLOAK_PORT="${KEYCLOAK_PORT:-19090}"
WEBAPI_PORT="${WEBAPI_PORT:-18084}"
TIMEOUT="${TIMEOUT:-60}"  # Shorter timeout for local testing

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Reproducing Pins BeforeAll Test Scenario"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check prerequisites
echo ""
echo "→ Checking prerequisites..."
kubectl get deployment -n "$NAMESPACE" nebari-landing-webapi &>/dev/null || {
    echo "✗ Webapi deployment not found in namespace $NAMESPACE"
    echo "  Run ./scripts/setup-local-e2e.sh first"
    exit 1
}

# Start Keycloak port-forward in background
echo ""
echo "→ Starting Keycloak port-forward on :$KEYCLOAK_PORT..."
kubectl port-forward -n keycloak svc/keycloak-keycloakx-http "$KEYCLOAK_PORT:80" &
KC_PF_PID=$!
trap "kill $KC_PF_PID 2>/dev/null || true" EXIT

sleep 3

# Get admin password
echo ""
echo "→ Getting Keycloak admin password..."
ADMIN_PASSWORD=$(kubectl get secret -n keycloak keycloak-keycloakx-admin \
    -o jsonpath='{.data.password}' | base64 -d)

# Get bearer token
echo ""
echo "→ Acquiring bearer token from Keycloak..."
TOKEN_RESPONSE=$(curl -s -X POST \
    "http://localhost:$KEYCLOAK_PORT/realms/nebari/protocol/openid-connect/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "grant_type=password" \
    -d "client_id=nebari-public" \
    -d "username=admin" \
    -d "password=$ADMIN_PASSWORD")

BEARER_TOKEN=$(echo "$TOKEN_RESPONSE" | jq -r '.access_token')
if [ -z "$BEARER_TOKEN" ] || [ "$BEARER_TOKEN" = "null" ]; then
    echo "✗ Failed to get bearer token"
    echo "Response: $TOKEN_RESPONSE"
    exit 1
fi
echo "✓ Got bearer token (${#BEARER_TOKEN} chars)"

# Create test NebariApp
echo ""
echo "→ Creating test NebariApp: test-pin-app..."
cat <<EOF | kubectl apply -f -
apiVersion: reconcilers.nebari.dev/v1
kind: NebariApp
metadata:
  name: test-pin-app
  namespace: $NAMESPACE
spec:
  hostname: test-pin-app.nebari.test
  service:
    name: test-service
    port: 8080
  landingPage:
    enabled: true
    displayName: "Test Service test-pin-app"
    description: "E2E test resource with visibility=public"
    category: "Testing"
    priority: 88
EOF

# Create backing service if it doesn't exist
echo ""
echo "→ Ensuring test-service exists..."
kubectl get service test-service -n "$NAMESPACE" &>/dev/null || {
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: test-service
  namespace: $NAMESPACE
spec:
  selector:
    app: test-dummy
  ports:
  - protocol: TCP
    port: 8080
    targetPort: 8080
EOF
}

# Start webapi port-forward
echo ""
echo "→ Starting webapi port-forward on :$WEBAPI_PORT..."
kubectl port-forward -n "$NAMESPACE" svc/nebari-landing-webapi "$WEBAPI_PORT:8080" &
WA_PF_PID=$!
trap "kill $WA_PF_PID $KC_PF_PID 2>/dev/null || true" EXIT

sleep 3

# Wait for webapi health
echo ""
echo "→ Waiting for webapi health..."
for i in $(seq 1 10); do
    if curl -sf "http://localhost:$WEBAPI_PORT/api/v1/health" >/dev/null; then
        echo "✓ Webapi healthy"
        break
    fi
    if [ $i -eq 10 ]; then
        echo "✗ Webapi not healthy after 10s"
        exit 1
    fi
    sleep 1
done

# Poll /api/v1/services looking for test-pin-app
echo ""
echo "→ Polling /api/v1/services for test-pin-app (timeout: ${TIMEOUT}s)..."
START_TIME=$(date +%s)
FOUND=false

while true; do
    ELAPSED=$(($(date +%s) - START_TIME))
    if [ $ELAPSED -ge $TIMEOUT ]; then
        echo ""
        echo "✗ TIMEOUT after ${ELAPSED}s - test-pin-app NOT found"
        break
    fi
    
    RESPONSE=$(curl -sf -H "Authorization: Bearer $BEARER_TOKEN" \
        "http://localhost:$WEBAPI_PORT/api/v1/services")
    
    # Check if test-pin-app is in the response
    if echo "$RESPONSE" | jq -e '.services[] | select(.displayName == "Test Service test-pin-app")' >/dev/null 2>&1; then
        echo ""
        echo "✓ FOUND test-pin-app after ${ELAPSED}s"
        echo ""
        echo "Service details:"
        echo "$RESPONSE" | jq '.services[] | select(.displayName == "Test Service test-pin-app")'
        FOUND=true
        break
    fi
    
    # Show progress
    printf "\r  Attempt %2ds: services count=%d" "$ELAPSED" \
        "$(echo "$RESPONSE" | jq '.services | length' 2>/dev/null || echo 0)"
    
    sleep 2
done

if [ "$FOUND" = "false" ]; then
    echo ""
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "FAILURE REPRODUCED"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "Current services in API response:"
    curl -sf -H "Authorization: Bearer $BEARER_TOKEN" \
        "http://localhost:$WEBAPI_PORT/api/v1/services" | jq '.'
    echo ""
    echo "Running diagnostics..."
    ./scripts/debug-e2e.sh
    exit 1
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test passed - service appeared in API"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
