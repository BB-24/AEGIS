PROJECT_NAME := aegis
VERSION ?= dev
BIN_DIR := bin
COMPONENTS := aegis-operator aegis-worker aegis-metadata aegis-storage aegis-cli
DOCKER_REGISTRY ?= ghcr.io
DOCKER_ORG ?= aegis-project
DOCKER_TAG ?= $(VERSION)
GOLANGCI_LINT_VERSION := v1.61.0
BUF_VERSION := v1.47.2
GORELEASER_VERSION := v2.4.8

.DEFAULT_GOAL := help
.PHONY: help build build-binaries test test-unit integration-test lint generate docker-build release release-dry-run clean install-tools

help: ## Show available targets
	@echo AEGIS build targets:
	@echo   build             Generate protobuf bindings and build all binaries
	@echo   test              Run unit and integration tests
	@echo   integration-test  Run Go tests tagged integration
	@echo   lint              Run Go and protobuf linters
	@echo   generate          Generate Go protobuf bindings with Buf
	@echo   docker-build      Build a Docker image for each component
	@echo   release           Publish a release with GoReleaser
	@echo   clean             Remove generated binaries and release artifacts

build: generate build-binaries ## Generate protobuf bindings and build all binaries

build-binaries: ## Build all Go binaries into bin/
	@mkdir -p $(BIN_DIR)
	@set -eu; for component in $(COMPONENTS); do \
		echo "Building $$component"; \
		CGO_ENABLED=0 go build -trimpath -o $(BIN_DIR)/$$component ./cmd/$$component; \
	done

test: test-unit integration-test ## Run unit and integration tests

test-unit: ## Run Go unit tests with the race detector
	go test -race -count=1 ./...

integration-test: ## Run Go tests tagged integration
	go test -race -count=1 -tags=integration ./...

lint: ## Run Go and protobuf linters
	golangci-lint run --timeout=5m ./...
	buf lint

generate: ## Generate Go protobuf bindings with Buf
	buf generate

docker-build: ## Build a Docker image for each component
	@set -eu; for component in $(COMPONENTS); do \
		echo "Building image for $$component"; \
		docker build \
			-t $(DOCKER_REGISTRY)/$(DOCKER_ORG)/$$component:$(DOCKER_TAG) \
			-f cmd/$$component/Dockerfile .; \
	done

release-dry-run: ## Create a local snapshot release
	goreleaser release --snapshot --clean

release: ## Publish a release (requires a valid GoReleaser token)
	goreleaser release --clean

install-tools: ## Install pinned Go development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
	go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

clean: ## Remove generated binaries and release artifacts
	rm -rf $(BIN_DIR) dist
