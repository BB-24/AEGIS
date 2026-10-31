# AEGIS - Makefile
# Orchestration for building, testing, and operating the AEGIS platform
#
# Usage:
#   make help              - Show this help
#   make build             - Build all binaries
#   make test              - Run all tests
#   make lint              - Run linters
#   make generate          - Generate code (protobuf, CRDs, mocks)
#   make docker-build      - Build Docker images
#   make deploy            - Deploy to Kubernetes (requires context)
#   make clean             - Clean build artifacts
#   make issues            - Render issue list from implementation plan

# ==============================================================================
# Configuration
# ==============================================================================

# Project metadata
PROJECT_NAME := aegis
PROJECT_ROOT := $(shell pwd)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go configuration
GO_VERSION := 1.23
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
GO_BUILD_FLAGS := -ldflags "\
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildTime=$(BUILD_TIME) \
	-s -w"
GO_TEST_FLAGS := -race -count=1 -timeout 30m
GO_COVER_FLAGS := -coverprofile=coverage.out -covermode=atomic

# Directories
BIN_DIR := $(PROJECT_ROOT)/bin
BUILD_DIR := $(PROJECT_ROOT)/build
DIST_DIR := $(PROJECT_ROOT)/dist
COVERAGE_DIR := $(PROJECT_ROOT)/coverage
DOCS_DIR := $(PROJECT_ROOT)/docs
SCRIPTS_DIR := $(PROJECT_ROOT)/scripts

# Binary names
BINARIES := aegis-operator aegis-worker aegis-metadata aegis-storage aegis-cli
BINARY_PATHS := $(addprefix $(BIN_DIR)/,$(BINARIES))

# Docker configuration
DOCKER_REGISTRY ?= ghcr.io
DOCKER_ORG ?= aegis-project
DOCKER_TAG ?= $(VERSION)
DOCKER_IMAGES := $(addprefix $(DOCKER_REGISTRY)/$(DOCKER_ORG)/,$(BINARIES))

# Kubernetes configuration
KUBECONFIG ?= ~/.kube/config
NAMESPACE ?= aegis-system

# Tool versions
GOLANGCI_LINT_VERSION := v1.61.0
PROTOC_VERSION := 27.3
CONTROLLER_GEN_VERSION := v0.16.4
KUSTOMIZE_VERSION := v5.4.3
HELM_VERSION := v3.14.0

# ==============================================================================
# Phony targets
# ==============================================================================

.PHONY: help build test lint generate docker-build deploy clean issues \
	build-binaries build-proto build-crds build-mocks \
	test-unit test-integration test-e2e test-chaos test-fuzz test-linearizability test-benchmarks \
	lint-go lint-proto lint-docker lint-yaml lint-markdown \
	generate-proto generate-crds generate-mocks generate-docs \
	docker-build-binaries docker-build-all docker-push \
	deploy-k8s deploy-helm deploy-argocd \
	clean-bin clean-build clean-coverage clean-docker \
	verify-prereqs check-go-version install-tools

# ==============================================================================
# Help
# ==============================================================================

help: ## Show this help message
	@echo "AEGIS - Immutable Distributed Data Protection Platform for Kubernetes"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*##"; printf "\n"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""

# ==============================================================================
# Prerequisites verification
# ==============================================================================

verify-prereqs: check-go-version ## Verify all required tools are installed
	@echo "Verifying prerequisites..."
	@which go >/dev/null || (echo "Go not found" && exit 1)
	@which docker >/dev/null || (echo "Docker not found" && exit 1)
	@which kubectl >/dev/null || (echo "kubectl not found" && exit 1)
	@which helm >/dev/null || (echo "Helm not found" && exit 1)
	@echo "All prerequisites satisfied"

check-go-version: ## Check Go version matches requirement
	@go_version=$$(go version | awk '{print $$3}' | sed 's/go//'); \
	required="$(GO_VERSION)"; \
	if [ "$$(printf '%s\n' "$$required" "$$go_version" | sort -V | head -n1)" != "$$required" ]; then \
		echo "Go version $$go_version < $(GO_VERSION) required"; exit 1; \
	fi; \
	echo "Go version $$go_version OK"

install-tools: ## Install development tools (golangci-lint, controller-gen, etc.)
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install github.com/golang/mock/mockgen@latest
	@echo "Tools installed to $$(go env GOPATH)/bin"

# ==============================================================================
# Build
# ==============================================================================

build: build-binaries build-proto build-crds ## Build all artifacts (binaries, proto, CRDs)

build-binaries: $(BINARY_PATHS) ## Build all Go binaries

$(BIN_DIR)/%: cmd/%/main.go
	@mkdir -p $(BIN_DIR)
	@echo "Building $*..."
	@CGO_ENABLED=0 go build $(GO_BUILD_FLAGS) -o $@ ./cmd/$*

build-proto: generate-proto ## Generate protobuf code

build-crds: generate-crds ## Generate CRD manifests

build-mocks: generate-mocks ## Generate mocks for testing

# ==============================================================================
# Code Generation
# ==============================================================================

generate: generate-proto generate-crds generate-mocks generate-docs ## Generate all code and manifests

generate-proto: ## Generate Go code from protobuf definitions
	@echo "Generating protobuf code..."
	@mkdir -p $(PROJECT_ROOT)/pkg/proto/generated
	@protoc --go_out=$(PROJECT_ROOT)/pkg/proto/generated \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(PROJECT_ROOT)/pkg/proto/generated \
		--go-grpc_opt=paths=source_relative \
		-I $(PROJECT_ROOT)/proto \
		$(PROJECT_ROOT)/proto/**/*.proto 2>/dev/null || true
	@echo "Protobuf generation complete"

generate-crds: ## Generate CRD manifests using controller-gen
	@echo "Generating CRD manifests..."
	@mkdir -p $(PROJECT_ROOT)/config/crd/bases
	@controller-gen crd:crdVersions=v1 \
		output:crd:artifacts:config=$(PROJECT_ROOT)/config/crd/bases \
		paths="./api/..."
	@echo "CRD generation complete"

generate-mocks: ## Generate mocks for interfaces
	@echo "Generating mocks..."
	@mkdir -p $(PROJECT_ROOT)/internal/mocks
	@find $(PROJECT_ROOT)/internal -name "*.go" -exec grep -l "interface" {} \; | \
		while read file; do \
			mockgen -source=$$file -destination=$(PROJECT_ROOT)/internal/mocks/$$(basename $$file .go)_mock.go -package=mocks; \
		done 2>/dev/null || true
	@echo "Mock generation complete"

generate-docs: ## Generate documentation from code
	@echo "Generating documentation..."
	@mkdir -p $(DOCS_DIR)/api
	@go run github.com/ahmetb/gen-crd-api-reference-docs@latest \
		-api-dir=$(PROJECT_ROOT)/api/... \
		-out-dir=$(DOCS_DIR)/api \
		-template-dir=$(PROJECT_ROOT)/hack/api-docs-template 2>/dev/null || true
	@echo "Documentation generation complete"

# ==============================================================================
# Testing
# ==============================================================================

test: test-unit test-integration ## Run all tests

test-unit: ## Run unit tests
	@echo "Running unit tests..."
	@mkdir -p $(COVERAGE_DIR)
	@go test $(GO_TEST_FLAGS) $(GO_COVER_FLAGS) ./internal/... ./pkg/... ./cmd/... 2>&1 | tee $(COVERAGE_DIR)/unit.log
	@go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/unit.html 2>/dev/null || true
	@echo "Unit tests complete"

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	@go test $(GO_TEST_FLAGS) -tags=integration ./tests/integration/... 2>&1 | tee $(COVERAGE_DIR)/integration.log
	@echo "Integration tests complete"

test-e2e: ## Run end-to-end tests (requires kind cluster)
	@echo "Running e2e tests..."
	@go test $(GO_TEST_FLAGS) -tags=e2e ./tests/e2e/... 2>&1 | tee $(COVERAGE_DIR)/e2e.log
	@echo "E2E tests complete"

test-chaos: ## Run chaos tests (requires kind cluster)
	@echo "Running chaos tests..."
	@go test $(GO_TEST_FLAGS) -tags=chaos ./tests/chaos/... 2>&1 | tee $(COVERAGE_DIR)/chaos.log
	@echo "Chaos tests complete"

test-fuzz: ## Run fuzz tests
	@echo "Running fuzz tests..."
	@go test $(GO_TEST_FLAGS) -fuzz=Fuzz -fuzztime=30s ./tests/fuzz/... 2>&1 | tee $(COVERAGE_DIR)/fuzz.log
	@echo "Fuzz tests complete"

test-linearizability: ## Run linearizability tests
	@echo "Running linearizability tests..."
	@go test $(GO_TEST_FLAGS) -tags=linearizability ./tests/linearizability/... 2>&1 | tee $(COVERAGE_DIR)/linearizability.log
	@echo "Linearizability tests complete"

test-benchmarks: ## Run benchmarks
	@echo "Running benchmarks..."
	@go test $(GO_TEST_FLAGS) -bench=. -benchmem ./tests/benchmarks/... 2>&1 | tee $(COVERAGE_DIR)/benchmarks.log
	@echo "Benchmarks complete"

# ==============================================================================
# Linting
# ==============================================================================

lint: lint-go lint-proto lint-docker lint-yaml lint-markdown ## Run all linters

lint-go: ## Run golangci-lint
	@echo "Running golangci-lint..."
	@golangci-lint run ./... --timeout=10m

lint-proto: ## Lint protobuf files
	@echo "Linting protobuf files..."
	@buf lint $(PROJECT_ROOT)/proto 2>/dev/null || echo "buf not installed, skipping proto lint"

lint-docker: ## Lint Dockerfiles
	@echo "Linting Dockerfiles..."
	@hadolint $(PROJECT_ROOT)/cmd/*/Dockerfile 2>/dev/null || echo "hadolint not installed, skipping Dockerfile lint"

lint-yaml: ## Lint YAML files
	@echo "Linting YAML files..."
	@yamllint $(PROJECT_ROOT)/config $(PROJECT_ROOT)/deploy 2>/dev/null || echo "yamllint not installed, skipping YAML lint"

lint-markdown: ## Lint Markdown files
	@echo "Linting Markdown files..."
	@markdownlint $(DOCS_DIR) 2>/dev/null || echo "markdownlint not installed, skipping Markdown lint"

# ==============================================================================
# Docker
# ==============================================================================

docker-build: docker-build-binaries ## Build all Docker images

docker-build-binaries: $(BINARY_PATHS) ## Build Docker images for all binaries
	@echo "Building Docker images..."
	@for bin in $(BINARIES); do \
		docker build -t $(DOCKER_REGISTRY)/$(DOCKER_ORG)/$$bin:$(DOCKER_TAG) \
			-f $(PROJECT_ROOT)/cmd/$$bin/Dockerfile \
			$(PROJECT_ROOT); \
	done
	@echo "Docker images built"

docker-build-all: docker-build-binaries ## Build all Docker images (alias)

docker-push: ## Push Docker images to registry
	@echo "Pushing Docker images..."
	@for bin in $(BINARIES); do \
		docker push $(DOCKER_REGISTRY)/$(DOCKER_ORG)/$$bin:$(DOCKER_TAG); \
	done
	@echo "Docker images pushed"

# ==============================================================================
# Deployment
# ==============================================================================

deploy: deploy-k8s ## Deploy to Kubernetes (alias for deploy-k8s)

deploy-k8s: ## Deploy using Kustomize
	@echo "Deploying to Kubernetes..."
	@kubectl apply -k $(PROJECT_ROOT)/deploy/k8s/overlays/production
	@echo "Deployment complete"

deploy-helm: ## Deploy using Helm
	@echo "Deploying with Helm..."
	@helm upgrade --install aegis $(PROJECT_ROOT)/deploy/helm/aegis \
		--namespace $(NAMESPACE) --create-namespace \
		--wait --timeout 10m
	@echo "Helm deployment complete"

deploy-argocd: ## Deploy Argo CD applications
	@echo "Deploying Argo CD applications..."
	@kubectl apply -k $(PROJECT_ROOT)/deploy/argocd
	@echo "Argo CD deployment complete"

# ==============================================================================
# Issue rendering
# ==============================================================================

issues: ## Render issue list from implementation plan
	@$(SCRIPTS_DIR)/render-issues.sh

issues-json: ## Render issue list as JSON
	@$(SCRIPTS_DIR)/render-issues.sh --json

issues-md: ## Render issue list as Markdown
	@$(SCRIPTS_DIR)/render-issues.sh --md

issues-csv: ## Render issue list as CSV
	@$(SCRIPTS_DIR)/render-issues.sh --csv

# ==============================================================================
# Cleanup
# ==============================================================================

clean: clean-bin clean-build clean-coverage clean-docker ## Clean all build artifacts

clean-bin: ## Clean binary artifacts
	@echo "Cleaning binaries..."
	@rm -rf $(BIN_DIR)
	@rm -rf $(BUILD_DIR)

clean-build: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf $(DIST_DIR)
	@go clean -cache -modcache -testcache

clean-coverage: ## Clean coverage artifacts
	@echo "Cleaning coverage..."
	@rm -rf $(COVERAGE_DIR)

clean-docker: ## Clean Docker images
	@echo "Cleaning Docker images..."
	@for bin in $(BINARIES); do \
		docker rmi $(DOCKER_REGISTRY)/$(DOCKER_ORG)/$$bin:$(DOCKER_TAG) 2>/dev/null || true; \
	done

# ==============================================================================
# Development helpers
# ==============================================================================

run-operator: build-binaries ## Run operator locally
	@$(BIN_DIR)/aegis-operator

run-worker: build-binaries ## Run worker locally
	@$(BIN_DIR)/aegis-worker

run-metadata: build-binaries ## Run metadata node locally
	@$(BIN_DIR)/aegis-metadata

run-storage: build-binaries ## Run storage node locally
	@$(BIN_DIR)/aegis-storage

run-cli: build-binaries ## Run CLI locally
	@$(BIN_DIR)/aegis-cli

dev-setup: install-tools verify-prereqs generate ## Full development environment setup

# ==============================================================================
# Release
# ==============================================================================

release-dry-run: ## Dry-run release (build, test, docker-build)
	@make clean
	@make build
	@make test
	@make docker-build
	@echo "Release dry-run complete"

release: release-dry-run docker-push ## Full release (build, test, docker-build, docker-push)
	@echo "Release complete"

# ==============================================================================
# Variables for CI/CD
# ==============================================================================

# Export variables for sub-makes and scripts
export PROJECT_NAME VERSION COMMIT BUILD_TIME
export GO_BUILD_FLAGS GO_TEST_FLAGS
export DOCKER_REGISTRY DOCKER_ORG DOCKER_TAG
export KUBECONFIG NAMESPACE