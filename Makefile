# Copyright 2026, OpenTeams.
# SPDX-License-Identifier: Apache-2.0

##@ General

.DEFAULT_GOAL := help

# The help target prints out all targets with their descriptions organised
# alphabetically. Make sure to keep a blank line after the `##@` and `##` lines.
.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

WEBAPI_IMG    ?= quay.io/nebari/nebari-webapi:latest
CONTAINER_TOOL ?= docker
BINARY         = bin/webapi

.PHONY: build
build: ## Build the webapi binary.
	go build -o $(BINARY) ./cmd

.PHONY: docker-build
docker-build: ## Build the webapi container image.
	$(CONTAINER_TOOL) build -t $(WEBAPI_IMG) .

.PHONY: docker-push
docker-push: ## Push the webapi container image.
	$(CONTAINER_TOOL) push $(WEBAPI_IMG)

.PHONY: docker-build-push
docker-build-push: docker-build docker-push ## Build and push the webapi container image.

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

##@ Testing

.PHONY: test
test: fmt vet ## Run unit tests.
	go test ./internal/... -count=1 -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -1

.PHONY: test-html
test-html: test ## Generate HTML coverage report.
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: test-e2e
test-e2e: ## Run e2e tests (requires a live cluster with the operator+CRDs installed).
	@echo "Prerequisites: operator CRDs installed, nebari-system namespace exists."
	USE_EXISTING_CLUSTER=$(USE_EXISTING_CLUSTER) WEBAPI_IMG=$(WEBAPI_IMG) \
		go test ./test/e2e -v -ginkgo.v -tags=e2e -timeout=30m

##@ Deploy

.PHONY: deploy
deploy: ## Apply the webapi manifest to the cluster.
	kubectl apply -f deploy/manifest.yaml

.PHONY: undeploy
undeploy: ## Remove the webapi manifest from the cluster.
	kubectl delete -f deploy/manifest.yaml --ignore-not-found

WEBAPI_DEV_IMG    ?= webapi:dev
KIND_CLUSTER      ?= nebari-operator-dev

.PHONY: dev
dev: ## Build image, load into Kind, and deploy for local testing.
	@echo "Building webapi image $(WEBAPI_DEV_IMG)..."
	$(CONTAINER_TOOL) build -t $(WEBAPI_DEV_IMG) .
	@echo "Loading image into Kind cluster '$(KIND_CLUSTER)'..."
	kind load docker-image $(WEBAPI_DEV_IMG) --name $(KIND_CLUSTER)
	@echo "Deploying webapi..."
	kubectl create namespace nebari-system --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f deploy/manifest.yaml
	kubectl set image deployment/webapi api=$(WEBAPI_DEV_IMG) -n nebari-system
	kubectl patch deployment webapi -n nebari-system --type=json \
		-p='[{"op":"replace","path":"/spec/template/spec/containers/0/imagePullPolicy","value":"Never"}]'
	kubectl rollout status deployment/webapi -n nebari-system --timeout=60s
	@echo ""
	@echo "✅ WebAPI deployed. Port-forward with:"
	@echo "  kubectl port-forward -n nebari-system svc/webapi 8080:8080"

.PHONY: pf
pf: ## Port-forward the webapi to localhost:8080.
	kubectl port-forward -n nebari-system svc/webapi 8080:8080
