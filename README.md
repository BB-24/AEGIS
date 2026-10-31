# AEGIS

AEGIS is a secure, distributed, cloud-native data storage and recovery platform. This repository contains the Go monorepo foundation and the service entry points.

## Requirements

- Go 1.22 or later
- GNU Make
- [Buf CLI](https://buf.build/docs/installation/) for protobuf generation
- Docker for container image builds
- `golangci-lint` v1.61.0 for linting

## Get started

```sh
git clone https://github.com/aegis/aegis.git
cd aegis
make generate
make build
make test
```

Run one of the service binaries locally:

```sh
AEGIS_LISTEN_ADDR=:8080 go run ./cmd/aegis-storage
```

Each service (`aegis-operator`, `aegis-worker`, `aegis-metadata`, and `aegis-storage`) serves `GET /healthz` and `GET /readyz`. The default bind address is `:8080`; set `AEGIS_LISTEN_ADDR` to override it. The CLI entry point is `cmd/aegis-cli`.

## Common targets

| Target | Description |
| --- | --- |
| `make build` | Generate protobuf code and build all five binaries into `bin/` |
| `make test` | Run all Go tests with the race detector |
| `make integration-test` | Run tests tagged `integration` under `tests/integration/` |
| `make lint` | Run `golangci-lint` and `buf lint` |
| `make generate` | Generate protobuf bindings with Buf |
| `make docker-build` | Build an image for every component |
| `make release` | Create release artifacts with GoReleaser |
| `make help` | List available targets |

Use `make install-tools` to install the pinned Go development tools. `go generate ./pkg/proto` runs the protobuf generator through the source directive. Configure a pre-commit installation with `pre-commit install`.

Each binary has its own multi-stage Dockerfile under `cmd/<component>/Dockerfile`. Images use a minimal non-root runtime image.

## Repository map

- `cmd/` — service and CLI entry points
- `internal/` — implementation-only runtime packages
- `pkg/` — public Go packages and generated API bindings
- `proto/` — protobuf service definitions
- `api/`, `config/`, `deploy/` — API and deployment configuration
- `tests/` — unit, integration, and system test suites
- `docs/` — architecture and implementation documentation

See [docs/repository-structure.md](docs/repository-structure.md) and [docs/implementation-plan.md](docs/implementation-plan.md) for additional project context.
