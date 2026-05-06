#!/usr/bin/env bash
# Setup local k3d cluster for e2e testing (mimics the sandbox action)
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-nebari-e2e-local}"
K8S_VERSION="${K8S_VERSION:-v1.32.4-k3s1}"
GITOPS_DIR="/tmp/nebari-gitops-${CLUSTER_NAME}"

echo "═══════════════════════════════════════════════════════════════════════"
echo " Setting up local Nebari E2E environment"
echo "═══════════════════════════════════════════════════════════════════════"
echo "Cluster: $CLUSTER_NAME"
echo "Kubernetes: $K8S_VERSION"
echo ""

# Check if cluster already exists
if k3d cluster list | grep -q "^${CLUSTER_NAME}"; then
    echo "✓ Cluster '${CLUSTER_NAME}' already exists"
    k3d kubeconfig merge "${CLUSTER_NAME}" --kubeconfig-switch-context
else
    echo "→ Creating k3d cluster..."
    k3d cluster create "${CLUSTER_NAME}" \
        --image "rancher/k3s:${K8S_VERSION}" \
        --agents 0 \
        --servers 1 \
        --k3s-arg "--disable=traefik@server:0" \
        --volume "${GITOPS_DIR}:/gitops@server:0" \
        --wait
    
    echo "✓ Cluster created"
fi

# Install NIC (Nebari Infrastructure CLI)
echo ""
echo "→ Installing NIC..."
if ! command -v nic &> /dev/null; then
    echo "  Downloading NIC binary..."
    curl -fsSL https://github.com/nebari-dev/nebari-infra-cli/releases/latest/download/nic_linux_amd64 \
        -o /tmp/nic
    chmod +x /tmp/nic
    sudo mv /tmp/nic /usr/local/bin/nic
    echo "✓ NIC installed"
else
    echo "✓ NIC already installed ($(nic --version))"
fi

# Deploy platform using NIC
echo ""
echo "→ Deploying Nebari platform stack (this takes ~10-15 minutes)..."
echo "  Components: MetalLB, cert-manager, Envoy Gateway, Keycloak, ArgoCD"
echo ""

nic local deploy \
    --cluster-name "${CLUSTER_NAME}" \
    --provider local \
    --profile platform \
    --gitops-dir "${GITOPS_DIR}" \
    --timeout 15m

echo ""
echo "✓ Platform deployed"

# Extract outputs
echo ""
echo "→ Extracting platform credentials..."

KEYCLOAK_PASS=$(kubectl -n keycloak get secret keycloak-keycloakx-credentials \
    -o jsonpath='{.data.admin-password}' | base64 -d 2>/dev/null || echo "")

ARGOCD_PASS=$(kubectl -n argocd get secret argocd-initial-admin-secret \
    -o jsonpath='{.data.password}' | base64 -d 2>/dev/null || echo "")

NEBARI_REALM_PASS=$(kubectl -n keycloak get secret nebari-realm-admin-credentials \
    -o jsonpath='{.data.password}' | base64 -d 2>/dev/null || echo "")

GATEWAY_IP=$(kubectl -n envoy-gateway-system get svc envoy-envoy-gateway-system-nebari-gateway-be66687c \
    -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")

# Wait for Keycloak nebari realm to be ready
echo ""
echo "→ Waiting for Keycloak nebari realm..."
kubectl port-forward -n keycloak svc/keycloak-keycloakx-http 19090:8080 &
PF_PID=$!
trap "kill ${PF_PID} 2>/dev/null || true" EXIT

for i in $(seq 1 60); do
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 \
        http://localhost:19090/realms/nebari/.well-known/openid-configuration 2>/dev/null || echo "000")
    if [ "$STATUS" = "200" ]; then
        echo "✓ Nebari realm ready"
        break
    fi
    if [ $i -eq 60 ]; then
        echo "✗ Timeout waiting for nebari realm"
        exit 1
    fi
    sleep 5
done

kill "${PF_PID}" 2>/dev/null || true
trap - EXIT

# Install NebariApp CRD
echo ""
echo "→ Installing NebariApp CRD..."
kubectl apply -f \
    https://raw.githubusercontent.com/nebari-dev/nebari-operator/main/config/crd/bases/reconcilers.nebari.dev_nebariapps.yaml

kubectl wait --for=condition=Established \
    crd/nebariapps.reconcilers.nebari.dev \
    --timeout=60s

echo "✓ NebariApp CRD installed"

# Build and load webapi image
echo ""
echo "→ Building webapi Docker image..."
cd "$(git rev-parse --show-toplevel)"

if docker images | grep -q "nebari-landing/webapi.*dev"; then
    echo "✓ Image already exists"
else
    docker build -t nebari-landing/webapi:dev .
    echo "✓ Image built"
fi

echo ""
echo "→ Loading webapi image into cluster..."
k3d image import nebari-landing/webapi:dev -c "${CLUSTER_NAME}"
echo "✓ Image loaded"

# Print summary
echo ""
echo "═══════════════════════════════════════════════════════════════════════"
echo " Local E2E Environment Ready!"
echo "═══════════════════════════════════════════════════════════════════════"
echo ""
echo "Cluster:              ${CLUSTER_NAME}"
echo "GitOps Dir:           ${GITOPS_DIR}"
echo "Gateway IP:           ${GATEWAY_IP}"
echo ""
echo "Credentials:"
echo "  Keycloak Admin:     admin / ${KEYCLOAK_PASS}"
echo "  ArgoCD Admin:       admin / ${ARGOCD_PASS}"
echo "  Nebari Realm Admin: admin / ${NEBARI_REALM_PASS}"
echo ""
echo "Export for tests:"
echo "  export USE_EXISTING_CLUSTER=true"
echo "  export CLUSTER_TYPE=k3d"
echo "  export NEBARI_REALM_ADMIN_PASSWORD='${NEBARI_REALM_PASS}'"
echo ""
echo "Run e2e tests:"
echo "  go test ./test/e2e/... -tags=e2e -v -timeout 20m"
echo ""
echo "Clean up:"
echo "  k3d cluster delete ${CLUSTER_NAME}"
echo "  rm -rf ${GITOPS_DIR}"
echo ""
