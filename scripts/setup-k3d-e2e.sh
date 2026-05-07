#!/usr/bin/env bash
#
# setup-k3d-e2e.sh - Minimal setup for e2e tests on k3d cluster
#
# This script sets up the minimum requirements for running e2e tests:
#  1. Install NebariApp CRD
#  2. Install Keycloak with nebari realm
#  3. Deploy webapi via Helm with custom image
#
# Prerequisites:
#  - k3d cluster running
#  - webapi-debug:local image loaded into k3d

set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-nebari-e2e-local}"
NAMESPACE="${NAMESPACE:-nebari-system}"
KC_NAMESPACE="${KC_NAMESPACE:-keycloak}"
WEBAPI_IMAGE="${WEBAPI_IMAGE:-webapi-debug:local}"
REALM_ADMIN_PASSWORD="${REALM_ADMIN_PASSWORD:-nebari-realm-admin}"

echo "=== Setting up k3d cluster for e2e tests ==="
echo "Cluster: ${CLUSTER_NAME}"
echo "Namespace: ${NAMESPACE}"
echo "Keycloak Namespace: ${KC_NAMESPACE}"
echo "Webapi Image: ${WEBAPI_IMAGE}"

# Check if cluster exists
if ! k3d cluster list | grep -q "${CLUSTER_NAME}"; then
    echo "Error: k3d cluster '${CLUSTER_NAME}' not found"
    exit 1
fi

# Ensure kubectl context is set
kubectl config use-context "k3d-${CLUSTER_NAME}"

# 1. Install NebariApp CRD
echo ""
echo "==> Installing NebariApp CRD..."
kubectl apply -f https://raw.githubusercontent.com/nebari-dev/nebari-operator/main/config/crd/bases/reconcilers.nebari.dev_nebariapps.yaml
kubectl wait --for=condition=Established crd/nebariapps.reconcilers.nebari.dev --timeout=60s
echo "✓ NebariApp CRD installed"

# 2. Install Keycloak
echo ""
echo "==> Installing Keycloak..."
helm repo add codecentric https://codecentric.github.io/helm-charts --force-update
helm repo update

# Create keycloak namespace
kubectl create namespace "${KC_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# Install PostgreSQL for Keycloak
helm upgrade --install keycloak-postgres oci://registry-1.docker.io/bitnamicharts/postgresql \
    --namespace "${KC_NAMESPACE}" \
    --set auth.postgresPassword=keycloak-db-secret \
    --set auth.database=keycloak \
    --wait --timeout=5m

# Install Keycloak
helm upgrade --install keycloak codecentric/keycloakx \
    --version 7.1.6 \
    --namespace "${KC_NAMESPACE}" \
    --set replicas=1 \
    --set database.vendor=postgres \
    --set database.hostname=keycloak-postgres-postgresql \
    --set database.port=5432 \
    --set database.database=keycloak \
    --set database.username=postgres \
    --set database.password=keycloak-db-secret \
    --set auth.adminUser=admin \
    --set auth.adminPassword=nebari-admin-secret \
    --set http.relativePath="/" \
    --set service.httpPort=8080 \
    --wait --timeout=10m

echo "✓ Keycloak installed"

# 3. Configure nebari realm
echo ""
echo "==> Configuring nebari realm in Keycloak..."

# Wait for Keycloak to be fully ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=keycloakx -n "${KC_NAMESPACE}" --timeout=5m

# Port-forward to Keycloak
kubectl port-forward -n "${KC_NAMESPACE}" svc/keycloak-keycloakx-http 18090:8080 &
PF_PID=$!
trap "kill ${PF_PID} 2>/dev/null || true" EXIT

# Wait for port-forward
sleep 5

# Get admin token
ADMIN_TOKEN=$(curl -s -X POST "http://localhost:18090/realms/master/protocol/openid-connect/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "username=admin" \
    -d "password=nebari-admin-secret" \
    -d "grant_type=password" \
    -d "client_id=admin-cli" | jq -r '.access_token')

if [ -z "$ADMIN_TOKEN" ] || [ "$ADMIN_TOKEN" = "null" ]; then
    echo "Error: Failed to get admin token from Keycloak"
    exit 1
fi

# Create nebari realm
echo "Creating nebari realm..."
curl -s -X POST "http://localhost:18090/admin/realms" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
        "realm": "nebari",
        "enabled": true,
        "displayName": "Nebari",
        "registrationAllowed": false,
        "loginWithEmailAllowed": true,
        "duplicateEmailsAllowed": false
    }'

# Create realm admin user
echo "Creating realm admin user..."
curl -s -X POST "http://localhost:18090/admin/realms/nebari/users" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{
        \"username\": \"admin\",
        \"enabled\": true,
        \"email\": \"admin@nebari.local\",
        \"emailVerified\": true,
        \"credentials\": [{
            \"type\": \"password\",
            \"value\": \"${REALM_ADMIN_PASSWORD}\",
            \"temporary\": false
        }]
    }"

# Store realm admin password in secret
kubectl create secret generic nebari-realm-admin-credentials \
    --from-literal=password="${REALM_ADMIN_PASSWORD}" \
    -n "${KC_NAMESPACE}" \
    --dry-run=client -o yaml | kubectl apply -f -

echo "✓ nebari realm configured"

# Kill port-forward
kill ${PF_PID} 2>/dev/null || true
trap - EXIT

# 4. Install webapi via Helm
echo ""
echo "==> Installing webapi via Helm..."

# Create app namespace
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# Install using chart with custom values
helm upgrade --install nebari-landing ./charts/nebari-landing \
    --namespace "${NAMESPACE}" \
    --set webapi.image.repository="webapi-debug" \
    --set webapi.image.tag="local" \
    --set webapi.image.pullPolicy="Never" \
    --set webapi.replicas=1 \
    --set webapi.keycloak.issuerURL="http://keycloak-keycloakx-http.${KC_NAMESPACE}.svc.cluster.local:8080" \
    --set webapi.keycloak.realm="nebari" \
    --set redis.enabled=true \
    --set frontend.enabled=false \
    --wait --timeout=5m

echo "✓ webapi installed"

echo ""
echo "=== Setup complete! ==="
echo ""
echo "To run e2e tests:"
echo "  export USE_EXISTING_CLUSTER=true"
echo "  export E2E_KEYCLOAK_ADMIN_PASSWORD=${REALM_ADMIN_PASSWORD}"
echo "  go test -tags=e2e ./test/e2e/... -v"
echo ""
echo "To view webapi logs:"
echo "  kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=webapi -f"
