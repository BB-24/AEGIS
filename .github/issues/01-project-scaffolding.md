# Issue #1: Project Scaffolding & Build System

**Epic**: Foundation | **Area**: Build | **Priority**: P0

## Description

Establish the monorepo structure, build system, and development tooling for the AEGIS project. This is the foundation for all subsequent work.

## Scope

### Repository Structure

```
aegis/
├── cmd/                    # Entry points
│   ├── aegis-operator/
│   ├── aegis-storage/
│   ├── aegis-metadata/
│   └── aegis-worker/
├── internal/               # Private packages
│   ├── chunker/           # FastCDC chunking
│   ├── crypto/            # Encryption primitives
│   ├── dedup/             # Deduplication
│   ├── compression/       # zstd compression
│   ├── erasure/           # Reed-Solomon coding
│   ├── storage/           # Storage node server
│   ├── metadata/          # Raft metadata
│   ├── operator/          # Kubernetes operator
│   ├── scheduler/         # Task scheduler
│   ├── backup/            # Backup workflow
│   ├── restore/           # Restore workflow
│   ├── security/          # SPIFFE, KMS, auth
│   └── observability/     # Metrics, tracing
├── pkg/                    # Public packages
├── proto/                  # Protobuf definitions
├── api/                    # Generated Go SDK
├── config/                 # Configuration
├── deploy/                 # K8s manifests, Helm charts
├── test/                   # Integration test utilities
├── docs/                   # Documentation
├── scripts/                # Build/test scripts
├── go.mod
├── go.sum
├── Makefile
├── .golangci.yml
├── .github/
│   ├── workflows/          # CI/CD pipelines
│   └── ISSUE_TEMPLATE/
└── README.md
```

### Build System

- **Go 1.22+** with modules
- **Makefile** with targets: `build`, `test`, `lint`, `generate`, `docker-build`, `release`
- **golangci-lint** configuration
- **protobuf** generation with `buf` or `protoc-gen-go`
- **GoReleaser** for release automation
- **Docker** multi-stage builds for each component

### Development Tooling

- `go generate` for code generation
- `make test` runs unit + race detector
- `make integration-test` for integration tests
- Pre-commit hooks for formatting and linting
- VS Code devcontainer configuration

### Configuration

- Environment-based configuration with `viper`/`koanf`
- Structured logging with `zerolog` or `slog`
- Feature flags for experimental features

## Acceptance Criteria

- [ ] Monorepo structure created with all directories
- [ ] `go.mod` initialized with module path `github.com/aegis/aegis`
- [ ] Makefile with all standard targets working
- [ ] golangci-lint passes with project config
- [ ] Protobuf generation works (`make generate`)
- [ ] Docker images build for all components (`make docker-build`)
- [ ] CI workflow runs on PR and push
- [ ] All components compile without errors
- [ ] Basic health check endpoint on each service

## Dependencies

- None (this is the first task)

## Definition of Done

- Repository structure matches specification
- All build targets execute successfully
- CI pipeline passes on main branch
- Developer onboarding documented in README

## References

- [Implementation Plan - Section 7.1](../docs/implementation-plan.md#71-project-scaffolding)
- [Architecture - Logical Architecture](../docs/implementation-plan.md#21-logical-architecture)