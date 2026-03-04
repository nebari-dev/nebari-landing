##############################################################################
# Nebari Landing – Local Dev Cluster Makefile
#
# Run all targets from the REPOSITORY ROOT:
#   make -f dev/Makefile <target>
# Or set a shell alias:  alias mk='make -f dev/Makefile'
#
# Targets (in recommended order):
#   make setup          – full end-to-end cluster + app setup
#   make cluster-start  – start/create the minikube cluster
#   make image-build    – build & load the container image into minikube
#   make cert-manager   – install cert-manager + selfsigned ClusterIssuer
#   make metallb        – enable MetalLB + auto-detect IP pool (eliminates port-forwards)
#   make minikube-tunnel – (WSL2 mirrored mode) inject routes so MetalLB IPs reach Windows
#   make keycloak       – install Keycloak with the nebari realm
#   make keycloak-client – create webapi + frontend OIDC clients in Keycloak
# make frontend-client – create the nebari-landingpage confidential OIDC client + k8s Secret
#   make operator-install – deploy operator + webapi via kustomize
#   make test-apps      – apply sample NebariApp CRs for local testing
#   make argocd-apps    – apply ArgoCD Application manifests directly
#   make install        – deploy nebari-landingpage (kustomize dev overlay)
#   make uninstall      – delete deployed resources
#   make cluster-delete – delete the minikube cluster
#   make info           – print useful URLs / credentials
#
# Prerequisites:
#   docker, kubectl, helm  (the Makefile downloads minikube automatically)
##############################################################################

# ── Configurable variables ──────────────────────────────────────────────────

CLUSTER_NAME       ?= nebari-local
MINIKUBE_CPUS      ?= 4
MINIKUBE_MEMORY    ?= 8192
MINIKUBE_DRIVER    ?= docker

IMAGE_REPO         ?= nebari-landing
IMAGE_TAG          ?= dev
IMAGE              := $(IMAGE_REPO):$(IMAGE_TAG)

HELM_RELEASE_KC    ?= keycloak

NAMESPACE_APP      ?= nebari-system
NAMESPACE_KC       ?= keycloak
NAMESPACE_OPERATOR ?= nebari-operator-system

# Keycloak realm (used in info output)
KEYCLOAK_REALM     ?= nebari

KEYCLOAK_CHART      := https://codecentric.github.io/helm-charts
KEYCLOAK_CHART_NAME := keycloakx
KEYCLOAK_VERSION    ?= 7.1.6
POSTGRESQL_CHART    := oci://registry-1.docker.io/bitnamicharts/postgresql
POSTGRESQL_VERSION  ?= 18.2.0

# Dev-only passwords — override via environment or .env file
KEYCLOAK_ADMIN_PASSWORD ?= nebari-admin-secret
KEYCLOAK_DB_PASSWORD    ?= keycloak-db-secret
REALM_ADMIN_PASSWORD    ?= nebari-realm-admin

# MetalLB pinned IPs.
# These default to the standard minikube docker-driver subnet for the 'nebari-local'
# profile.  The 'metallb' target auto-detects the actual subnet at runtime via
# 'minikube ip', so these variables are used only for the 'info' output and for
# the optional service annotations wired into Helm/kustomize overlays.
# If you rename the cluster via CLUSTER_NAME the auto-detection handles it;
# only override these manually if you use a non-docker minikube driver.
LB_IP_KEYCLOAK  ?= 192.168.49.100
LB_IP_WEBAPI    ?= 192.168.49.101
LB_IP_LANDING   ?= 192.168.49.102

# ── Port-forward mode ────────────────────────────────────────────────────────
# Services are exposed as NodePort. 'make port-forward' (run automatically by
# 'make setup') binds localhost:PORT to each service via kubectl port-forward.
PORT_KEYCLOAK  ?= 8180
PORT_WEBAPI    ?= 8090
PORT_LANDING   ?= 8080
# Optional production domain (e.g. nebari.example.com) — leave empty for local-only installs.
# Set via: make frontend-client LANDING_HOST=nebari.example.com
LANDING_HOST   ?=

OVERLAY            ?= dev

# ── Detect OS / Arch ────────────────────────────────────────────────────────

OS   := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)
ifeq ($(ARCH),x86_64)
  ARCH := amd64
endif
ifeq ($(ARCH),aarch64)
  ARCH := arm64
endif

# ── Binary paths (local, not required to be in PATH) ────────────────────────

BIN_DIR       := $(CURDIR)/.bin
MINIKUBE      := $(BIN_DIR)/minikube
MINIKUBE_URL  := https://storage.googleapis.com/minikube/releases/latest/minikube-$(OS)-$(ARCH)

# ── Phony targets ────────────────────────────────────────────────────────────

.PHONY: setup cluster-start cluster-delete \
        image-build \
        cert-manager metallb \
        postgresql postgresql-secrets \
        keycloak keycloak-install keycloak-realm keycloak-client frontend-client \
        operator-install operator-uninstall \
        test-apps \
        argocd-apps install uninstall dev-watch stop-dev-watch info \
        port-forward stop-port-forward \
        minikube-tunnel stop-minikube-tunnel \
        deps-check _minikube

OVERLAY_PATH := dev/manifests/nebari-landingpage/overlays/$(OVERLAY)

# ── Main entry point ─────────────────────────────────────────────────────────

## setup: full end-to-end setup (cluster + image + keycloak + app)
setup: _minikube cluster-start image-build cert-manager metallb keycloak keycloak-client frontend-client operator-install install test-apps port-forward info
	@echo ""
	@echo "✅  Setup complete. Run 'make info' at any time for URLs."

# ── Dependency download ───────────────────────────────────────────────────────

## _minikube: download minikube binary if not already present
_minikube:
	@if command -v minikube > /dev/null 2>&1; then \
	  MINIKUBE=$$(command -v minikube); \
	  echo "✔  minikube found at $$MINIKUBE (version: $$(minikube version --short 2>/dev/null))"; \
	else \
	  mkdir -p $(BIN_DIR); \
	  if [ ! -x $(MINIKUBE) ]; then \
	    echo "→  Downloading minikube for $(OS)/$(ARCH) …"; \
	    curl -fsSL $(MINIKUBE_URL) -o $(MINIKUBE); \
	    chmod +x $(MINIKUBE); \
	    echo "✔  minikube saved to $(MINIKUBE)"; \
	  else \
	    echo "✔  minikube already downloaded at $(MINIKUBE)"; \
	  fi; \
	fi

## deps-check: verify required tools are installed
deps-check:
	@echo "Checking required tools …"
	@command -v docker  > /dev/null || (echo "✗  docker not found"  && exit 1)
	@command -v kubectl > /dev/null || (echo "✗  kubectl not found" && exit 1)
	@command -v helm    > /dev/null || (echo "✗  helm not found"    && exit 1)
	@echo "✔  docker, kubectl, helm found"

# ── Cluster lifecycle ─────────────────────────────────────────────────────────

# Resolve the minikube binary (system or local .bin/)
MINIKUBE_BIN := $(shell command -v minikube 2>/dev/null || echo $(MINIKUBE))

## cluster-start: create (or start) the minikube cluster
cluster-start:
	@echo "→  Starting minikube cluster '$(CLUSTER_NAME)' …"
	$(MINIKUBE_BIN) start \
	  --profile $(CLUSTER_NAME) \
	  --cpus $(MINIKUBE_CPUS) \
	  --memory $(MINIKUBE_MEMORY) \
	  --driver $(MINIKUBE_DRIVER) \
	  --addons=ingress,metrics-server
	@echo "✔  Cluster ready"
	$(MINIKUBE_BIN) profile $(CLUSTER_NAME)

## cluster-delete: delete the minikube cluster
cluster-delete:
	$(MINIKUBE_BIN) delete --profile $(CLUSTER_NAME)

# ── Image build & load ────────────────────────────────────────────────────────

## image-build: build the Docker image and load it directly into minikube
image-build:
	@echo "→  Building container image $(IMAGE) …"
	$(MINIKUBE_BIN) -p $(CLUSTER_NAME) image build \
	  -t $(IMAGE) \
	  -f dev/Dockerfile \
	  .
	@echo "✔  Image $(IMAGE) loaded into minikube"

# ── cert-manager ───────────────────────────────────────────────────────────────────

CERT_MANAGER_VERSION ?= v1.16.3

## cert-manager: install cert-manager and a selfsigned ClusterIssuer
cert-manager:
	@echo "→  Installing cert-manager $(CERT_MANAGER_VERSION) …"
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml
	kubectl rollout status deployment/cert-manager         -n cert-manager --timeout=3m
	kubectl rollout status deployment/cert-manager-webhook -n cert-manager --timeout=3m
	kubectl apply -f dev/manifests/cert-manager/selfsigned-clusterissuer.yaml
	@echo "✔  cert-manager ready with selfsigned ClusterIssuer"

# ── MetalLB ───────────────────────────────────────────────────────────────────

## metallb: enable MetalLB minikube addon and configure the local IP pool
## Auto-detects the minikube node subnet from 'minikube ip' at runtime so the
## pool is always valid regardless of CLUSTER_NAME or driver-assigned subnet.
## On WSL2 (mirrored mode) also run 'make minikube-tunnel' so Windows gets routes.
metallb:
	@echo "→  Enabling MetalLB addon …"
	$(MINIKUBE_BIN) addons enable metallb -p $(CLUSTER_NAME)
	@echo "→  Waiting for MetalLB controller to become ready …"
	kubectl rollout status deployment/controller -n metallb-system --timeout=3m
	@MINIKUBE_IP=$$($(MINIKUBE_BIN) ip -p $(CLUSTER_NAME)); \
	 SUBNET=$$(echo "$$MINIKUBE_IP" | sed 's/\.[0-9]*$$//'); \
	 POOL_START=$${SUBNET}.100; \
	 POOL_END=$${SUBNET}.150; \
	 echo "→  Detected minikube subnet: $${SUBNET}.0/24"; \
	 echo "→  Configuring IP pool $${POOL_START}-$${POOL_END} …"; \
	 printf 'apiVersion: v1\nkind: ConfigMap\nmetadata:\n  namespace: metallb-system\n  name: config\ndata:\n  config: |\n    address-pools:\n    - name: nebari-local\n      protocol: layer2\n      addresses:\n      - %s-%s\n' \
	   "$${POOL_START}" "$${POOL_END}" | kubectl apply -f -; \
	 echo "✔  MetalLB ready. Pinned IPs:"; \
	 echo "     Keycloak : $${SUBNET}.100 (override: LB_IP_KEYCLOAK)"; \
	 echo "     WebAPI   : $${SUBNET}.101 (override: LB_IP_WEBAPI)"; \
	 echo "     Landing  : $${SUBNET}.102 (override: LB_IP_LANDING)"; \
	 echo ""; \
	 echo "     NOTE (WSL2 mirrored mode): the docker bridge is not automatically"; \
	 echo "     routable from Windows even in mirrored mode. Run:"; \
	 echo "       make -f dev/Makefile minikube-tunnel"; \
	 echo "     to inject routes into the Windows networking stack."

## keycloak: install PostgreSQL + Keycloak and configure the nebari realm
keycloak: postgresql keycloak-install keycloak-realm
	@echo "✔  Keycloak ready with realm 'nebari'"

## postgresql-secrets: create the k8s secrets required by PostgreSQL and Keycloak
postgresql-secrets:
	kubectl create namespace $(NAMESPACE_KC) --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic keycloak-admin-credentials \
	  --from-literal=admin-password=$(KEYCLOAK_ADMIN_PASSWORD) \
	  --namespace $(NAMESPACE_KC) \
	  --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic keycloak-postgresql-credentials \
	  --from-literal=password=$(KEYCLOAK_DB_PASSWORD) \
	  --namespace $(NAMESPACE_KC) \
	  --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic nebari-realm-admin-credentials \
	  --from-literal=password=$(REALM_ADMIN_PASSWORD) \
	  --namespace $(NAMESPACE_KC) \
	  --dry-run=client -o yaml | kubectl apply -f -

## postgresql: install PostgreSQL (external DB for Keycloak)
postgresql: postgresql-secrets
	@echo "→  Installing PostgreSQL …"
	helm upgrade --install postgresql $(POSTGRESQL_CHART) \
	  --version $(POSTGRESQL_VERSION) \
	  --namespace $(NAMESPACE_KC) \
	  --values dev/keycloak/postgresql-values.yaml \
	  --wait --timeout 10m
	@echo "✔  PostgreSQL deployed"

keycloak-install:
	@echo "→  Installing Keycloak (codecentric/keycloakx) …"
	kubectl create namespace $(NAMESPACE_KC) --dry-run=client -o yaml | kubectl apply -f -
	helm repo add codecentric $(KEYCLOAK_CHART) --force-update
	helm upgrade --install $(HELM_RELEASE_KC) codecentric/$(KEYCLOAK_CHART_NAME) \
	  --version $(KEYCLOAK_VERSION) \
	  --namespace $(NAMESPACE_KC) \
	  --values dev/keycloak/values.yaml \
	  --wait --timeout 10m
	@echo "✔  Keycloak deployed"

## keycloak-realm: run the kcadm Job to create and populate the nebari realm
keycloak-realm:
	@echo "→  Running realm setup Job …"
	kubectl delete job keycloak-realm-setup -n $(NAMESPACE_KC) --ignore-not-found
	kubectl apply -f dev/keycloak/realm-setup-job.yaml
	kubectl wait --for=condition=complete job/keycloak-realm-setup \
	  -n $(NAMESPACE_KC) --timeout=5m
	@echo "✔  Realm 'nebari' configured"

## keycloak-client: create the webapi public OIDC client in the nebari realm
keycloak-client:
	@echo "→  Creating webapi Keycloak client …"
	kubectl delete job keycloak-webapi-client-setup -n $(NAMESPACE_KC) --ignore-not-found
	kubectl apply -f dev/keycloak/webapi-client-job.yaml
	kubectl wait --for=condition=complete job/keycloak-webapi-client-setup \
	  -n $(NAMESPACE_KC) --timeout=3m
	@echo "✔  Keycloak webapi client ready"

## frontend-client: create the nebari-landingpage confidential OIDC client (dev fallback)
## In production the nebari-operator provisions this automatically from spec.auth
## on the 'nebari-landingpage' NebariApp. In dev, the operator exits early at the
## TLS step (envoy-gateway-system missing) and never reaches auth provisioning.
## This target replicates what the operator would do:
##   1. Creates the Keycloak client (kcadm Job)
##   2. Writes the same Secret the operator would create: nebari-landingpage-oidc-client
frontend-client:
	@echo "→  Creating nebari-landingpage Keycloak client (dev fallback) …"
	kubectl delete job keycloak-frontend-client-setup -n $(NAMESPACE_KC) --ignore-not-found
	PORT_LANDING=$(PORT_LANDING) LANDING_HOST=$(LANDING_HOST) envsubst '$${PORT_LANDING} $${LANDING_HOST}' < dev/keycloak/frontend-client-job.yaml | kubectl apply -f -
	kubectl wait --for=condition=complete job/keycloak-frontend-client-setup \
	  -n $(NAMESPACE_KC) --timeout=3m
	@echo "→  Writing nebari-landingpage-oidc-client Secret (mirrors operator output) …"
	kubectl create namespace $(NAMESPACE_APP) --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic nebari-landingpage-oidc-client \
	  --namespace $(NAMESPACE_APP) \
	  --from-literal=client-id=nebari-landingpage \
	  --from-literal=client-secret=nebari-frontend-dev-secret \
	  --dry-run=client -o yaml | kubectl apply -f -
	@echo "✔  Keycloak nebari-landingpage client ready"

# ── ArgoCD Application manifests ─────────────────────────────────────────────
# These are plain Kubernetes CRs (argoproj.io/v1alpha1 Application).
# The files contain Go-template placeholders ({{ .GitRepoURL }} etc.) that
# must be substituted before applying.  Use envsubst or your infrastructure
# templating tool, then run: kubectl apply -f <rendered-file>
#
# Example with envsubst:
#   export GIT_REPO_URL=https://github.com/nebari-dev/nebari-landing
#   export GIT_BRANCH=main
#   export GIT_PATH=""
#   envsubst < manifests/argocd/nebari-landingpage-app.yaml | kubectl apply -f -

# ── Operator + WebAPI ────────────────────────────────────────────────────────
# Uses kustomize overlays that pull resources directly from the nebari-operator
# repo (remote URL).  Edit the kustomization.yaml files under
#   dev/manifests/nebari-operator/operator/
#   dev/manifests/nebari-operator/webapi/
# to change image tags or add further patches.

## operator-install: deploy the nebari-operator + webapi via kustomize
operator-install:
	@echo "→  Deploying nebari-operator …"
	kubectl create namespace $(NAMESPACE_OPERATOR) --dry-run=client -o yaml | kubectl apply -f -
	kubectl create namespace $(NAMESPACE_APP)      --dry-run=client -o yaml | kubectl apply -f -
	kubectl label namespace $(NAMESPACE_APP) nebari.dev/managed=true --overwrite
	kubectl apply -k dev/manifests/nebari-operator/operator
	kubectl apply -k dev/manifests/nebari-operator/webapi
	kubectl rollout status deployment/nebari-operator-controller-manager -n $(NAMESPACE_OPERATOR) --timeout=5m
	kubectl rollout status deployment/webapi                             -n $(NAMESPACE_APP)      --timeout=5m
	@echo "✔  nebari-operator + webapi deployed"

## operator-uninstall: remove the operator and webapi deployments
operator-uninstall:
	-kubectl delete -k dev/manifests/nebari-operator/operator --ignore-not-found
	-kubectl delete -k dev/manifests/nebari-operator/webapi   --ignore-not-found
	@echo "✔  nebari-operator + webapi removed"

## test-apps: apply sample NebariApp CRs to populate the webapi service cache
test-apps:
	@echo "→  Applying test NebariApp resources …"
	kubectl apply -f dev/manifests/test-nebariapps.yaml
	@echo "✔  Test NebariApps applied"

## argocd-apps: apply the rendered ArgoCD Application manifests
argocd-apps:
	@echo "→  Applying ArgoCD Application manifests …"
	kubectl apply -f dev/manifests/argocd/
	@echo "✔  ArgoCD Application manifests applied"

# ── Landing page ──────────────────────────────────────────────────────────────

## install: deploy nebari-landingpage using the kustomize overlay (default: dev)
install:
	@echo "→  Applying kustomize overlay '$(OVERLAY)' …"
	kubectl create namespace $(NAMESPACE_APP) --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -k $(OVERLAY_PATH)
	kubectl rollout status deployment/nebari-landingpage -n $(NAMESPACE_APP) --timeout=3m
	@echo "✔  nebari-landingpage deployed in namespace '$(NAMESPACE_APP)'"

## uninstall: remove deployed resources (does not delete the cluster)
uninstall: operator-uninstall
	-kubectl delete -k $(OVERLAY_PATH) --ignore-not-found
	-kubectl delete job keycloak-realm-setup -n $(NAMESPACE_KC) --ignore-not-found
	-helm uninstall $(HELM_RELEASE_KC) -n $(NAMESPACE_KC)
	-helm uninstall postgresql           -n $(NAMESPACE_KC)

## dev-watch: swap the landing page to a Vite HMR dev server with local frontend/ mounted
## File changes on the host are picked up immediately via minikube mount → Vite HMR.
## The browser hot-reloads without a pod restart.
## Requires a running cluster ('make setup'). Stop with: make -f dev/Makefile stop-dev-watch
dev-watch:
	@echo "→  Starting minikube mount in a new terminal …"
	@pkill -f "minikub[e].*mount.*nebari-frontend" 2>/dev/null || true
	@sleep 1
	# Open a dedicated terminal for the mount process — it must stay alive.
	# Uses nohup + setsid to fully detach from the current process group.
	@setsid nohup $(MINIKUBE_BIN) -p $(CLUSTER_NAME) mount \
	  "$(CURDIR)/frontend:/mnt/nebari-frontend" \
	  > /tmp/minikube-mount-frontend.log 2>&1 < /dev/null &
	@echo "   Mount PID written to /tmp/minikube-mount-frontend.pid"
	@echo $$! > /tmp/minikube-mount-frontend.pid
	@echo "   Waiting for mount to establish …"
	@for i in $$(seq 1 15); do \
	  if grep -q "Watching" /tmp/minikube-mount-frontend.log 2>/dev/null; then \
	    echo "✔  minikube mount active (log: /tmp/minikube-mount-frontend.log)"; break; \
	  fi; \
	  sleep 1; \
	done
	@echo "→  Applying dev-watch overlay (Vite HMR + hostPath volume) …"
	$(MAKE) -f dev/Makefile install OVERLAY=dev-watch
	$(MAKE) -f dev/Makefile port-forward
	@echo ""
	@echo "✔  Dev-watch active. Edit files in frontend/ — Vite HMR reloads the browser."
	@echo "   Landing  : http://localhost:$(PORT_LANDING)/"
	@echo "   Mount log: /tmp/minikube-mount-frontend.log"
	@echo "   Stop     : make -f dev/Makefile stop-dev-watch"

## stop-dev-watch: restore the prebuilt nginx image and stop the minikube mount
stop-dev-watch:
	@echo "→  Restoring standard dev overlay (nginx + prebuilt image) …"
	$(MAKE) -f dev/Makefile install OVERLAY=dev
	$(MAKE) -f dev/Makefile port-forward
	@pkill -f "minikub[e].*mount.*nebari-frontend" 2>/dev/null && echo "✔  minikube mount stopped" || true
	@echo "✔  dev-watch stopped — run 'make -f dev/Makefile image-build' to rebuild the image"

# ── Info / convenience ────────────────────────────────────────────────────────

## info: print service URLs and credentials
info:
	@echo ""
	@echo "╔══════════════════════════════════════════════════════════════╗"
	@echo "║              Nebari Local Cluster – Quick Info               ║"
	@echo "╠══════════════════════════════════════════════════════════════╣"
	@echo "║  Cluster profile : $(CLUSTER_NAME)"
	@MINIKUBE_IP=$$($(MINIKUBE_BIN) ip -p $(CLUSTER_NAME) 2>/dev/null || echo ""); \
	 SUBNET=$$(echo "$$MINIKUBE_IP" | sed 's/\.[0-9]*$$//'); \
	 KC_IP=$${SUBNET:-192.168.49}.100; \
	 API_IP=$${SUBNET:-192.168.49}.101; \
	 LAND_IP=$${SUBNET:-192.168.49}.102; \
	 echo "║  Cluster IP      : $$MINIKUBE_IP"; \
	 echo "║  MetalLB subnet   : $${SUBNET:-192.168.49}.0/24 (pool .100–.150)"; \
	 echo "║"; \
	 echo "║  MetalLB LoadBalancer IPs:"; \
	 echo "║    (WSL2: also run 'make minikube-tunnel' for Windows access)"; \
	 echo "║"; \
	 echo "║  Landing page + oauth2-proxy:"; \
	 echo "║    http://$${LAND_IP}/"; \
	 echo "║    SPA token endpoint: http://$${LAND_IP}/auth-token"; \
	 echo "║"; \
	 echo "║  WebAPI:"; \
	 echo "║    http://$${API_IP}:8080/api/v1/services"; \
	 echo "║    http://$${API_IP}:8080/api/v1/health"; \
	 echo "║"; \
	 echo "║  Keycloak admin console:"; \
	 echo "║    http://$${KC_IP}/auth/admin  (admin / $(KEYCLOAK_ADMIN_PASSWORD))"; \
	 echo "║    Realm: $(KEYCLOAK_REALM)  /  test user: admin / $(REALM_ADMIN_PASSWORD)"; \
	 echo "║"; \
	 echo "║  Operator namespace : $(NAMESPACE_OPERATOR)"; \
	 echo "║"; \
	 echo "║  Integration tests:"; \
	 echo "║    python3 dev/webapi_test.py \\\""; \
	 echo "║      --webapi-url http://$${API_IP}:8080 \\\""; \
	 echo "║      --keycloak-url http://$${KC_IP}/auth \\\""; \
	 echo "║      -u admin -p $(REALM_ADMIN_PASSWORD)"
	@echo "╚══════════════════════════════════════════════════════════════╝"
	@echo ""

## port-forward: forward Keycloak, WebAPI, and landing page to localhost
## Runs three kubectl port-forward processes in the background.
## Kill them with: make stop-port-forward
port-forward:
	@echo "→  (Re)starting port-forwards in the background …"
	@pkill -f "kubect[l].*port-forward.*keycloak-keycloakx-http" 2>/dev/null || true
	@pkill -f "kubect[l].*port-forward.*svc/webapi" 2>/dev/null || true
	@pkill -f "kubect[l].*port-forward.*nebari-landingpage" 2>/dev/null || true
	@nohup kubectl -n $(NAMESPACE_KC)  port-forward svc/keycloak-keycloakx-http $(PORT_KEYCLOAK):80  > /tmp/pf-keycloak.log 2>&1 &
	@nohup kubectl -n $(NAMESPACE_APP) port-forward svc/webapi                  $(PORT_WEBAPI):8080  > /tmp/pf-webapi.log 2>&1 &
	@nohup kubectl -n $(NAMESPACE_APP) port-forward svc/nebari-landingpage       $(PORT_LANDING):80   > /tmp/pf-landing.log 2>&1 &
	@sleep 2
	@echo "✔  Port-forwards active (logs: /tmp/pf-*.log):"
	@echo "     Keycloak   : http://localhost:$(PORT_KEYCLOAK)/auth"
	@echo "     WebAPI     : http://localhost:$(PORT_WEBAPI)/api/v1/services"
	@echo "     Landing    : http://localhost:$(PORT_LANDING)/"

## stop-port-forward: kill all background kubectl port-forward processes
stop-port-forward:
	@pkill -f "kubect[l].*port-forward.*keycloak-keycloakx-http" 2>/dev/null && echo "✔  Killed Keycloak port-forward" || true
	@pkill -f "kubect[l].*port-forward.*svc/webapi" 2>/dev/null && echo "✔  Killed WebAPI port-forward" || true
	@pkill -f "kubect[l].*port-forward.*nebari-landingpage" 2>/dev/null && echo "✔  Killed landing port-forward" || true

## minikube-tunnel: run 'minikube tunnel' in the background (needed on WSL2 mirrored mode)
## 'minikube tunnel' injects host routes for LoadBalancer IPs into the Linux kernel.
## In WSL2 mirrored networking mode those routes propagate to Windows, making
## 192.168.49.100-102 reachable from the Windows browser without port-forwarding.
## Must be left running for the duration of the dev session (sudo may be required).
minikube-tunnel:
	@echo "→  Starting minikube tunnel in the background …"
	@pkill -f "minikube.*tunnel" 2>/dev/null || true
	@$(MINIKUBE_BIN) tunnel -p $(CLUSTER_NAME) > /tmp/minikube-tunnel.log 2>&1 &
	@sleep 2
	@echo "✔  minikube tunnel running (log: /tmp/minikube-tunnel.log)"
	@echo "     If prompted for sudo password, run in a separate terminal:"
	@echo "       sudo $(MINIKUBE_BIN) tunnel -p $(CLUSTER_NAME)"

## stop-minikube-tunnel: kill the background minikube tunnel process
stop-minikube-tunnel:
	@pkill -f "minikube.*tunnel" 2>/dev/null && echo "✔  Killed minikube tunnel" || echo "minikube tunnel was not running"

## help: list all documented targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
