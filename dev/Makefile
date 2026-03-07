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
#   make image-build    – build webapi + frontend images and load into minikube
#   make cert-manager   – install cert-manager + selfsigned ClusterIssuer
#   make metallb        – enable MetalLB + auto-detect IP pool (eliminates port-forwards)
#   make minikube-tunnel – (WSL2 mirrored mode) inject routes so MetalLB IPs reach Windows
#   make keycloak       – install Keycloak with the nebari realm
#   make keycloak-client – create webapi + frontend OIDC clients in Keycloak
# make frontend-client – create the nebari-landingpage confidential OIDC client + k8s Secret
#   make operator-install – deploy nebari-operator controller (CRDs + controller manager)
#   make test-apps      – apply sample NebariApp CRs for local testing
#   make argocd-apps    – apply ArgoCD Application manifests directly
#   make install        – deploy nebari-landing via Helm chart (webapi + frontend + Redis)
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

IMAGE_REPO          ?= nebari-landing
IMAGE_TAG           ?= dev
WEBAPI_IMAGE        := $(IMAGE_REPO)/webapi:$(IMAGE_TAG)
FRONTEND_IMAGE      := $(IMAGE_REPO)/nebari-landing:$(IMAGE_TAG)
HELM_RELEASE        ?= nebari-landing

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
# docker-compose dev stack ports
PORT_MOCKAPI   ?= 8090
PORT_FRONTEND  ?= 5173
# Optional production domain (e.g. nebari.example.com) — leave empty for local-only installs.
# Set via: make frontend-client LANDING_HOST=nebari.example.com
LANDING_HOST   ?=

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
        argocd-apps install uninstall dev-watch stop-dev-watch dev-watch-status info \
        port-forward stop-port-forward \
        minikube-tunnel stop-minikube-tunnel \
        deps-check _minikube

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

## image-build: build webapi and frontend images and load them into minikube
# _minikube-reload: evict pods, remove stale image, load fresh one, restore replicas.
# $(1) = deployment name suffix (e.g. webapi, frontend)
# $(2) = pod label selector (e.g. app.kubernetes.io/component=webapi)
# $(3) = image ref (e.g. $(WEBAPI_IMAGE))
define _minikube-reload
	@REPLICAS=$$(kubectl get deploy $(HELM_RELEASE)-$(1) -n $(NAMESPACE_APP) \
	  -o jsonpath='{.spec.replicas}' 2>/dev/null || echo 0); \
	if [ "$$REPLICAS" -gt 0 ] 2>/dev/null; then \
	  echo "→  Scaling $(HELM_RELEASE)-$(1) to 0 to release the stale image …"; \
	  kubectl scale deployment $(HELM_RELEASE)-$(1) -n $(NAMESPACE_APP) --replicas=0; \
	  kubectl wait --for=delete pod -l $(2) -n $(NAMESPACE_APP) --timeout=60s 2>/dev/null || true; \
	fi; \
	echo "→  Removing old $(3) from minikube (if present) …"; \
	$(MINIKUBE_BIN) -p $(CLUSTER_NAME) image rm $(3) 2>/dev/null || true; \
	echo "→  Loading $(3) into minikube …"; \
	$(MINIKUBE_BIN) -p $(CLUSTER_NAME) image load $(3); \
	if [ "$$REPLICAS" -gt 0 ] 2>/dev/null; then \
	  echo "→  Restoring $(HELM_RELEASE)-$(1) to $$REPLICAS replica(s) …"; \
	  kubectl scale deployment $(HELM_RELEASE)-$(1) -n $(NAMESPACE_APP) --replicas=$$REPLICAS; \
	fi
endef

image-build:
	@echo "→  Building webapi image $(WEBAPI_IMAGE) …"
	docker build -t $(WEBAPI_IMAGE) -f Dockerfile .
	$(call _minikube-reload,webapi,app.kubernetes.io/component=webapi,$(WEBAPI_IMAGE))
	@echo "→  Building frontend image $(FRONTEND_IMAGE) …"
	# Build context is frontend/ — Dockerfile lives there and only touches that tree.
	docker build -t $(FRONTEND_IMAGE) -f frontend/Dockerfile frontend/
	$(call _minikube-reload,frontend,app.kubernetes.io/component=frontend,$(FRONTEND_IMAGE))
	@echo "✔  Images loaded into minikube: $(WEBAPI_IMAGE) $(FRONTEND_IMAGE)"

# ── cert-manager ───────────────────────────────────────────────────────────────────

CERT_MANAGER_VERSION ?= v1.16.3

## cert-manager: install cert-manager and a selfsigned ClusterIssuer
cert-manager:
	@echo "→  Installing cert-manager $(CERT_MANAGER_VERSION) …"
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml
	kubectl rollout status deployment/cert-manager              -n cert-manager --timeout=3m
	kubectl rollout status deployment/cert-manager-webhook       -n cert-manager --timeout=3m
	kubectl rollout status deployment/cert-manager-cainjector   -n cert-manager --timeout=3m
	@echo "→  Waiting for cert-manager webhook TLS to be ready …"
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
	  kubectl apply -f dev/manifests/cert-manager/selfsigned-clusterissuer.yaml 2>&1 && break; \
	  echo "  webhook not ready yet (attempt $$i/10), retrying in 5 s …"; \
	  sleep 5; \
	done
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
	@MINIKUBE_IP=$$($(MINIKUBE_BIN) ip -p $(CLUSTER_NAME)); \
	 SUBNET=$$(echo "$$MINIKUBE_IP" | sed 's/\.[0-9]*$$//'); \
	 export KC_HOSTNAME_URL="http://localhost:8180/auth"; \
	 echo "→  Keycloak public URL (KC_HOSTNAME_URL): $$KC_HOSTNAME_URL"; \
	 envsubst < dev/keycloak/values.yaml | \
	 helm upgrade --install $(HELM_RELEASE_KC) codecentric/$(KEYCLOAK_CHART_NAME) \
	   --version $(KEYCLOAK_VERSION) \
	   --namespace $(NAMESPACE_KC) \
	   --values - \
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
##   2. Writes nebari-landing-oauth2-proxy Secret (client-secret + cookie-secret)
##      consumed by the Helm chart's oauth2-proxy sidecar
frontend-client:
	@echo "→  Creating nebari-landingpage Keycloak client (dev fallback) …"
	kubectl delete job keycloak-frontend-client-setup -n $(NAMESPACE_KC) --ignore-not-found
	PORT_LANDING=$(PORT_LANDING) LANDING_HOST=$(LANDING_HOST) envsubst '$${PORT_LANDING} $${LANDING_HOST}' < dev/keycloak/frontend-client-job.yaml | kubectl apply -f -
	kubectl wait --for=condition=complete job/keycloak-frontend-client-setup \
	  -n $(NAMESPACE_KC) --timeout=3m
	@echo "→  Writing nebari-landing-oauth2-proxy Secret (Helm chart oauth2-proxy input) …"
	kubectl create namespace $(NAMESPACE_APP) --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic nebari-landing-oauth2-proxy \
	  --namespace $(NAMESPACE_APP) \
	  --from-literal=client-secret=nebari-frontend-dev-secret \
	  --from-literal=cookie-secret=$$(openssl rand -hex 16) \
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

## operator-install: deploy only the nebari-operator controller via kustomize
## The webapi is now managed by the Helm chart (make install)
operator-install:
	@echo "→  Deploying nebari-operator controller …"
	kubectl create namespace $(NAMESPACE_OPERATOR) --dry-run=client -o yaml | kubectl apply -f -
	kubectl create namespace $(NAMESPACE_APP)      --dry-run=client -o yaml | kubectl apply -f -
	kubectl label namespace $(NAMESPACE_APP) nebari.dev/managed=true --overwrite
	kubectl apply -k dev/manifests/nebari-operator/operator
	kubectl rollout status deployment/nebari-operator-controller-manager -n $(NAMESPACE_OPERATOR) --timeout=5m
	@echo "✔  nebari-operator controller deployed"

## operator-uninstall: remove the operator controller
operator-uninstall:
	-kubectl delete -k dev/manifests/nebari-operator/operator --ignore-not-found
	@echo "✔  nebari-operator controller removed"

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

## install: deploy nebari-landing via Helm chart (webapi + frontend + Redis)
install:
	@echo "→  Deploying nebari-landing Helm chart (release: $(HELM_RELEASE)) …"
	kubectl create namespace $(NAMESPACE_APP) --dry-run=client -o yaml | kubectl apply -f -
	helm dependency build charts/nebari-landing
	helm upgrade --install $(HELM_RELEASE) charts/nebari-landing \
	  --namespace $(NAMESPACE_APP) \
	  --values dev/chart-values.yaml \
	  --wait --timeout 5m
	# Force pods to restart so pullPolicy:Never always picks up the freshly loaded :dev image.
	# This is a no-op on first install (pods already started above) but essential on re-runs.
	-kubectl rollout restart deployment/$(HELM_RELEASE)-webapi    -n $(NAMESPACE_APP) 2>/dev/null || true
	-kubectl rollout restart deployment/$(HELM_RELEASE)-frontend  -n $(NAMESPACE_APP) 2>/dev/null || true
	-kubectl rollout status  deployment/$(HELM_RELEASE)-webapi    -n $(NAMESPACE_APP) --timeout=2m 2>/dev/null || true
	-kubectl rollout status  deployment/$(HELM_RELEASE)-frontend  -n $(NAMESPACE_APP) --timeout=2m 2>/dev/null || true
	@echo "✔  nebari-landing deployed in namespace '$(NAMESPACE_APP)'"

## uninstall: remove deployed resources (does not delete the cluster)
uninstall: operator-uninstall
	-helm uninstall $(HELM_RELEASE) -n $(NAMESPACE_APP)
	-kubectl delete job keycloak-realm-setup -n $(NAMESPACE_KC) --ignore-not-found
	-helm uninstall $(HELM_RELEASE_KC) -n $(NAMESPACE_KC)
	-helm uninstall postgresql           -n $(NAMESPACE_KC)

## dev-watch: swap the Helm-managed frontend pod to a Vite HMR dev server.
## Applies dev/patches/frontend-dev-watch.yaml via 'kubectl patch --type=strategic'
## to the existing nebari-landing-frontend Deployment — no parallel deployment,
## no kustomize overlay, no Helm chart modification.
## File changes on the host are picked up immediately via minikube mount → Vite HMR.
## Requires a running cluster ('make install'). Stop with: make -f dev/Makefile stop-dev-watch
dev-watch:
	@echo "→  Starting minikube mount …"
	@pkill -f "minikub[e].*mount.*nebari-frontend" 2>/dev/null || true
	@sleep 1
	# Launch the mount in a new OS session so it survives Make exiting.
	@python3 -c "\
import subprocess, shlex, sys; \
cmd = shlex.split('$(MINIKUBE_BIN) -p $(CLUSTER_NAME) mount $(CURDIR)/frontend:/mnt/nebari-frontend'); \
log = open('/tmp/minikube-mount-frontend.log', 'w'); \
p = subprocess.Popen(cmd, stdout=log, stderr=log, stdin=subprocess.DEVNULL, start_new_session=True); \
open('/tmp/minikube-mount-frontend.pid', 'w').write(str(p.pid)); \
print('   Mount PID', p.pid, '— log: /tmp/minikube-mount-frontend.log'); \
"
	@echo "   Waiting for mount to establish …"
	@for i in $$(seq 1 15); do \
	  if grep -q "Successfully mounted" /tmp/minikube-mount-frontend.log 2>/dev/null; then \
	    echo "✔  minikube mount active"; break; \
	  fi; \
	  sleep 1; \
	done
	@if ! grep -q "Successfully mounted" /tmp/minikube-mount-frontend.log 2>/dev/null; then \
	  echo "✗  minikube mount did not establish within 15 s — check /tmp/minikube-mount-frontend.log"; \
	  cat /tmp/minikube-mount-frontend.log; \
	  exit 1; \
	fi
	@echo "→  Patching Helm deployment to Vite HMR (dev/patches/frontend-dev-watch.yaml) …"
	kubectl patch deployment $(HELM_RELEASE)-frontend \
	  -n $(NAMESPACE_APP) \
	  --type=strategic \
	  --patch-file dev/patches/frontend-dev-watch.yaml
	@echo "   Deleting pods so the new spec takes effect immediately …"
	@kubectl -n $(NAMESPACE_APP) delete pod -l app.kubernetes.io/component=frontend 2>/dev/null || true
	@echo "   Waiting for Vite pod to become ready (~2-3 min for npm install + Vite startup) …"
	kubectl rollout status deployment/$(HELM_RELEASE)-frontend -n $(NAMESPACE_APP) --timeout=8m
	@pkill -f "kubect[l].*port-forward.*$(HELM_RELEASE)-frontend" 2>/dev/null || true
	@nohup kubectl -n $(NAMESPACE_APP) port-forward svc/$(HELM_RELEASE)-frontend $(PORT_LANDING):4180 \
	  > /tmp/pf-landing.log 2>&1 &
	@sleep 1
	@echo ""
	@echo "✔  Dev-watch active. Edit files in frontend/ — Vite HMR reloads the browser."
	@echo "   Landing  : http://localhost:$(PORT_LANDING)/"
	@echo "   Mount log: /tmp/minikube-mount-frontend.log"
	@echo "   Status   : make -f dev/Makefile dev-watch-status"
	@echo "   Stop     : make -f dev/Makefile stop-dev-watch"

## stop-dev-watch: restore the Helm-rendered nginx frontend and stop the minikube mount.
## Deletes the patched deployment first so helm upgrade recreates it cleanly (a plain
## 'helm upgrade' 3-way-merges and preserves strategic-patch additions like extra init
## containers and volumes, which prevents the nginx pod from starting).
stop-dev-watch:
	@echo "→  Stopping minikube mount …"
	@pkill -f "minikub[e].*mount.*nebari-frontend" 2>/dev/null && echo "✔  minikube mount stopped" || true
	@echo "→  Deleting patched frontend deployment so helm recreates it cleanly …"
	@kubectl delete deployment $(HELM_RELEASE)-frontend -n $(NAMESPACE_APP) --ignore-not-found
	@echo "→  Restoring Helm-managed nginx frontend (helm upgrade) …"
	helm upgrade $(HELM_RELEASE) charts/nebari-landing \
	  --namespace $(NAMESPACE_APP) \
	  --values dev/chart-values.yaml \
	  --wait --timeout 5m
	@echo "✔  dev-watch stopped — nginx frontend restored at http://localhost:$(PORT_LANDING)/"
	@echo "   Run 'make -f dev/Makefile image-build' to rebuild the prebuilt image if needed."

## dev-watch-status: show the health of all dev-watch components in one shot
## Checks: minikube mount process, mount log tail, pod phase, init + main container
## logs (last 30 lines each), and whether Vite is listening on :8080.
dev-watch-status:
	@echo ""
	@echo "╔══════════════════════════════════════════════════════════════╗"
	@echo "║                  dev-watch status                           ║"
	@echo "╠══════════════════════════════════════════════════════════════╣"
	@echo ""
	@echo "── 1. minikube mount process ──────────────────────────────────"
	@if pgrep -f "minikub[e].*mount.*nebari-frontend" > /dev/null 2>&1; then \
	  PID=$$(pgrep -f "minikub[e].*mount.*nebari-frontend" | head -1); \
	  echo "✔  Running (PID $$PID)"; \
	else \
	  echo "✗  NOT running — start with: make -f dev/Makefile dev-watch"; \
	fi
	@echo ""
	@echo "── 2. mount log (last 20 lines) ───────────────────────────────"
	@if [ -f /tmp/minikube-mount-frontend.log ]; then \
	  tail -n 20 /tmp/minikube-mount-frontend.log; \
	else \
	  echo "   (log file not found: /tmp/minikube-mount-frontend.log)"; \
	fi
	@echo ""
	@echo "── 3. pod status ──────────────────────────────────────────────"
	@kubectl -n $(NAMESPACE_APP) get pods -l app.kubernetes.io/component=frontend -o wide 2>/dev/null || echo "   (no pods found)"
	@echo ""
	@echo "── 4. init container logs (wait-for-mount, last 30 lines) ─────"
	@POD=$$(kubectl -n $(NAMESPACE_APP) get pods -l app.kubernetes.io/component=frontend \
	          --field-selector=status.phase=Running \
	          -o jsonpath='{.items[0].metadata.name}' 2>/dev/null); \
	 if [ -n "$$POD" ]; then \
	   kubectl -n $(NAMESPACE_APP) logs $$POD -c wait-for-mount --tail=30 2>/dev/null \
	     || echo "   (init container has already completed — no logs)"; \
	 else \
	   echo "   (no Running pod found)"; \
	 fi
	@echo ""
	@echo "── 5. Vite container logs (frontend, last 40 lines) ───────────"
	@POD=$$(kubectl -n $(NAMESPACE_APP) get pods -l app.kubernetes.io/component=frontend \
	          --field-selector=status.phase=Running \
	          -o jsonpath='{.items[0].metadata.name}' 2>/dev/null); \
	 if [ -n "$$POD" ]; then \
	   kubectl -n $(NAMESPACE_APP) logs $$POD -c frontend --tail=40 2>/dev/null; \
	 else \
	   echo "   (no Running pod found)"; \
	 fi
	@echo ""
	@echo "── 6. HMR reachability (port-forward → :8080) ─────────────────"
	@if curl -sf --max-time 3 http://localhost:$(PORT_LANDING)/ -o /dev/null; then \
	  echo "✔  http://localhost:$(PORT_LANDING)/ is reachable"; \
	else \
	  echo "✗  http://localhost:$(PORT_LANDING)/ unreachable — Vite may still be starting"; \
	fi
	@echo ""
	@echo "╚══════════════════════════════════════════════════════════════╝"
	@echo ""

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
	 echo "║    python3 dev/webapi_test.py \\"; \
	 echo "║      --webapi-url http://$${API_IP}:8080 \\"; \
	 echo "║      --keycloak-url http://$${KC_IP}/auth \\"; \
	 echo "║      -u admin -p $(REALM_ADMIN_PASSWORD)"; \
	 echo "║"; \
	 echo "║  Hot-reload dev (Vite HMR via minikube mount):"; \
	 if pgrep -f "minikub[e].*mount.*nebari-frontend" > /dev/null 2>&1; then \
	   PID=$$(pgrep -f "minikub[e].*mount.*nebari-frontend" | head -1); \
	   echo "║    ✔  minikube mount active (PID $$PID)"; \
	   echo "║       http://localhost:$(PORT_LANDING)/  ← Vite dev server"; \
	   echo "║       Log: /tmp/minikube-mount-frontend.log"; \
	   echo "║       Stop: make -f dev/Makefile stop-dev-watch"; \
	 else \
	   echo "║    ○  Not running  (nginx/prebuilt image active)"; \
	   echo "║       Start: make -f dev/Makefile dev-watch"; \
	   echo "║       Status: make -f dev/Makefile dev-watch-status"; \
	 fi
	@echo "╚══════════════════════════════════════════════════════════════╝"
	@echo ""

## port-forward: forward Keycloak, WebAPI, and landing page to localhost
## Runs three kubectl port-forward processes in the background.
## Kill them with: make stop-port-forward
port-forward:
	@echo "→  (Re)starting port-forwards in the background …"
	@pkill -f "kubect[l].*port-forward.*keycloak-keycloakx-http" 2>/dev/null || true
	@pkill -f "kubect[l].*port-forward.*(nebari-landing-webapi|nebari-landing-frontend)" 2>/dev/null || true
	@nohup kubectl -n $(NAMESPACE_KC)  port-forward svc/keycloak-keycloakx-http        $(PORT_KEYCLOAK):80   > /tmp/pf-keycloak.log 2>&1 &
	@nohup kubectl -n $(NAMESPACE_APP) port-forward svc/$(HELM_RELEASE)-webapi          $(PORT_WEBAPI):8080  > /tmp/pf-webapi.log 2>&1 &
	@nohup kubectl -n $(NAMESPACE_APP) port-forward svc/$(HELM_RELEASE)-frontend        $(PORT_LANDING):4180 > /tmp/pf-landing.log 2>&1 &
	@sleep 2
	@echo "✔  Port-forwards active (logs: /tmp/pf-*.log):"
	@echo "     Keycloak   : http://localhost:$(PORT_KEYCLOAK)/auth"
	@echo "     WebAPI     : http://localhost:$(PORT_WEBAPI)/api/v1/services"
	@echo "     Landing    : http://localhost:$(PORT_LANDING)/"

## stop-port-forward: kill all background kubectl port-forward processes
stop-port-forward:
	@pkill -f "kubect[l].*port-forward.*keycloak-keycloakx-http" 2>/dev/null && echo "✔  Killed Keycloak port-forward" || true
	@pkill -f "kubect[l].*port-forward.*(nebari-landing-webapi|nebari-landing-frontend)" 2>/dev/null && echo "✔  Killed app port-forwards" || true

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

# ── docker-compose dev stack ──────────────────────────────────────────────────
# Lightweight alternative to the full minikube cluster for frontend development.
# No Kubernetes required — just Docker.
#
# Services:
#   keycloak   http://localhost:8180/auth  (admin / nebari-admin-secret)
#   mockapi    http://localhost:8090       (reads test-nebariapps.yaml from disk)
#   frontend   http://localhost:5173       (Vite HMR, bind-mount from ./frontend)
#
# Run all three:     make -f dev/Makefile compose-up
# Skip the frontend container (run npm run dev locally instead):
#   make -f dev/Makefile compose-up-api
# Then in a separate terminal:
#   VITE_API_URL=http://localhost:8090 npm --prefix frontend run dev

COMPOSE_FILE := dev/docker-compose.yaml

.PHONY: compose-up compose-up-api compose-down compose-build compose-logs compose-info

## compose-up: start the full docker-compose dev stack (keycloak + mockapi + frontend)
compose-up:
	@echo "→  Starting docker-compose dev stack …"
	docker compose -f $(COMPOSE_FILE) up --build -d
	@$(MAKE) -f dev/Makefile compose-info

## compose-up-api: start only keycloak + mockapi (run frontend locally with npm)
compose-up-api:
	@echo "→  Starting keycloak + mockapi …"
	docker compose -f $(COMPOSE_FILE) up --build -d keycloak mockapi
	@echo ""
	@echo "✔  Backend services running. Start the frontend locally:"
	@echo "     VITE_API_URL=http://localhost:8090 npm --prefix frontend run dev"
	@echo ""
	@$(MAKE) -f dev/Makefile compose-info

## compose-down: stop and remove all compose containers
compose-down:
	docker compose -f $(COMPOSE_FILE) down
	@echo "✔  docker-compose stack stopped"

## compose-build: rebuild images without starting
compose-build:
	docker compose -f $(COMPOSE_FILE) build

## compose-logs: tail logs from all compose services
compose-logs:
	docker compose -f $(COMPOSE_FILE) logs -f

## compose-info: print URLs and credentials for the docker-compose stack
compose-info:
	@echo ""
	@echo "╔══════════════════════════════════════════════════════════════╗"
	@echo "║           Nebari docker-compose Dev Stack                    ║"
	@echo "╠══════════════════════════════════════════════════════════════╣"
	@echo "║  Keycloak admin console:                                     ║"
	@echo "║    http://localhost:$(PORT_KEYCLOAK)/auth/admin              ║"
	@echo "║    master admin : admin / $${KEYCLOAK_ADMIN_PASSWORD:-nebari-admin-secret}"
	@echo "║    Realm: nebari                                              ║"
	@echo "║      user: admin / nebari-realm-admin  (group: /admin)       ║"
	@echo "║      user: user  / nebari-user-secret  (group: /users)       ║"
	@echo "║                                                               ║"
	@echo "║  Mock API:                                                    ║"
	@echo "║    http://localhost:$(PORT_MOCKAPI)/api/v1/services           ║"
	@echo "║    http://localhost:$(PORT_MOCKAPI)/api/v1/health             ║"
	@echo "║                                                               ║"
	@echo "║  Frontend (Vite HMR):                                         ║"
	@echo "║    http://localhost:$(PORT_FRONTEND)/                          ║"
	@echo "║    (or run locally: VITE_API_URL=http://localhost:$(PORT_MOCKAPI) npm --prefix frontend run dev)"
	@echo "║                                                               ║"
	@echo "║  Stop stack: make -f dev/Makefile compose-down               ║"
	@echo "╚══════════════════════════════════════════════════════════════╝"
	@echo ""
