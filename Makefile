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
.PHONY: help build build-binaries test test-unit test-integration integration-test lint generate docker-build release release-dry-run clean install-tools dev-setup run-operator run-worker run-metadata run-storage run-cli

help: ## Show available targets
	@echo AEGIS build targets:
	@echo   build             Generate protobuf bindings and build all binaries
	@echo   test              Run unit and integration tests
	@echo   test-integration  Run integration tests
	@echo   integration-test  Run Go tests tagged integration
	@echo   lint              Run Go and protobuf linters
	@echo   generate          Generate Go protobuf bindings with Buf
	@echo   docker-build      Build a Docker image for each component
	@echo   release           Publish a release with GoReleaser
	@echo   clean             Remove generated binaries and release artifacts
	@echo   dev-setup         Full development environment setup
	@echo   run-operator      Run operator locally
	@echo   run-worker        Run worker locally
	@echo   run-metadata      Run metadata node locally
	@echo   run-storage       Run storage node locally
	@echo   run-cli           Run CLI locally

build: generate build-binaries ## Generate protobuf bindings and build all binaries

build-binaries: ## Build all Go binaries into bin/
	@mkdir -p $(BIN_DIR)
	@set -eu; for component in $(COMPONENTS); do \
		echo "Building $$component"; \
		CGO_ENABLED=0 go build -trimpath -o $(BIN_DIR)/$$component ./cmd/$$component; \
	done

test: test-unit test-integration ## Run unit and integration tests

test-unit: ## Run Go unit tests with the race detector
	go test -race -count=1 ./...

test-integration: ## Run integration tests
	docker compose -f docker-compose.test.yml up -d
	@sleep 10
	go test -race -count=1 -tags=integration ./...
	docker compose -f docker-compose.test.yml down -v

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

dev-setup: install-tools generate ## Full development environment setup
	@echo "Development environment ready!"

run-operator: build ## Run operator locally
	AEGIS_LISTEN_ADDR=:8080 go run ./cmd/aegis-operator

run-worker: build ## Run worker locally
	AEGIS_LISTEN_ADDR=:8080 go run ./cmd/aegis-worker

run-metadata: build ## Run metadata node locally
	AEGIS_LISTEN_ADDR=:8080 go run ./cmd/aegis-metadata

run-storage: build ## Run storage node locally
	AEGIS_LISTEN_ADDR=:8080 go run ./cmd/aegis-storage

run-cli: build ## Run CLI locally
	go run ./cmd/aegis-cli

clean: ## Remove generated binaries and release artifacts
	rm -rf $(BIN_DIR) dist
