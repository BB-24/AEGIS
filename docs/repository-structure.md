# AEGIS Repository Structure

This guide explains the repository scaffold, what belongs in each area, and where its tests and deployment assets live. The folders have been created as an initial skeleton; `.gitkeep` files retain empty directories until implementation files are added. File names below are planned ownership locations, not claims that those implementations already exist.

## Repository tree

```text
AEGIS/
├── .github/
│   └── workflows/                 # ci.yml, integration.yml, security.yml, sbom.yml, release.yml, provenance.yml, deploy.yml, chaos.yml
├── api/
│   └── v1alpha1/                  # backup_policy.go, backup.go, restore.go, groupversion_info.go, zz_generated.deepcopy.go
├── cmd/
│   ├── aegis-operator/            # main.go: Kubernetes operator entry point
│   ├── aegis-worker/              # main.go: backup/restore data mover entry point
│   ├── aegis-metadata/            # main.go: Raft metadata node entry point
│   ├── aegis-storage/             # main.go: shard storage node entry point
│   └── aegis-cli/                 # main.go: administrative and diagnostic CLI
├── config/
│   ├── crd/bases/                 # generated BackupPolicy, Backup, Restore CRD schemas
│   ├── rbac/                      # service accounts, roles, bindings, manager permissions
│   ├── security/
│   │   ├── kyverno/               # image/provenance and workload policies
│   │   ├── opa/                   # Rego policies, if OPA is selected alongside/instead of Kyverno
│   │   └── spire/                 # SPIRE server/agent and registration examples
│   ├── helm/aegis/                # Helm chart for AEGIS components
│   └── manifests/                 # Kustomize bases and generated Kubernetes manifests
├── deploy/
│   ├── k8s/                       # development and production Kubernetes overlays
│   ├── helm/aegis/                # chart packaging/release location, if kept separate from config source
│   └── argocd/                    # Argo CD Application and project definitions
├── docs/
│   ├── implementation-plan.md    # phased engineering plan and acceptance criteria
│   ├── repository-structure.md   # this directory and file ownership guide
│   └── runbooks/                  # operator, backup, restore, upgrade, incident, and DR procedures
├── internal/
│   ├── blobstore/                 # BlobStore interface and filesystem/S3 adapters
│   ├── backup/                    # backup workflow orchestration
│   ├── chunker/                   # streaming FastCDC implementation and chunk-boundary tests
│   ├── compression/               # compression algorithms and codec wrappers
│   ├── crypto/                    # SHA-256, zstd wrappers, AES-GCM, manifest signing primitives
│   ├── dedup/                     # content index and reference accounting logic
│   ├── erasure/                   # Reed-Solomon (6,3) encode/decode implementation
│   ├── health/                    # HTTP health and readiness endpoints
│   ├── metadata/                  # Raft node, state machine, durable store, snapshots, read path
│   ├── operator/                  # controllers, scheduler, reconciliation, worker assignment
│   ├── restore/                   # manifest verification, shard retrieval, restore and readiness logic
│   ├── scheduler/                 # task scheduling and worker assignment
│   ├── storage/                   # storage gRPC server, placement, shard I/O, repair
│   ├── service/                   # shared service startup and graceful shutdown
│   ├── task/                      # task lifecycle, leases, heartbeats, checkpoints
│   ├── security/                  # authentication, authorization, approvals, audit events
│   ├── kms/                       # KMS/Vault interfaces and provider adapters
│   ├── gc/                        # mark, tombstone, grace-period, and sweep logic
│   ├── recovery/                  # resume, repair orchestration, catalogue-based disaster recovery
│   └── observability/             # Prometheus metrics, OpenTelemetry, structured logging
├── pkg/
│   ├── client/                    # supported clients for metadata, storage, and Kubernetes APIs
│   ├── types/                     # shared public/domain types and error definitions
│   ├── util/                      # small cross-cutting helpers (retry, IDs, context)
│   └── proto/generated/           # generated Go protobuf and gRPC code; do not hand-edit
├── proto/
│   ├── api/                       # public/control API protobuf definitions
│   ├── metadata/                  # metadata and Raft client service contracts
│   ├── storage/                   # shard storage service contracts
│   └── worker/                    # worker task and progress contracts
├── scripts/                       # reproducible developer, codegen, test, SBOM, and signing scripts
├── tests/
│   ├── unit/                      # cross-package focused unit tests, where package-local tests do not fit
│   ├── integration/               # data-plane, storage, metadata, and workflow integration tests
│   ├── e2e/                       # Kubernetes/PostgreSQL backup-restore and DR demonstrations
│   ├── chaos/                     # fault-injection scenarios and harness configuration
│   ├── fuzz/                      # fuzz targets for parsers, chunking, crypto wrappers, and metadata
│   ├── linearizability/           # Porcupine histories and Raft consistency tests
│   └── benchmarks/                # throughput, latency, scaling, repair, and GC benchmarks
├── go.mod                         # Go module and direct dependency declarations
├── go.sum                         # pinned Go dependency checksums
├── Makefile                       # common test, lint, generate, build, and image targets
├── README.md                      # project overview, local development, and quick-start
├── SECURITY.md                    # vulnerability reporting and security policy
└── .gitignore                     # generated files, local data, credentials, and build output
```

## Folder ownership and file guidance

### `.github/workflows/`

Owns automated verification and release automation. Keep one workflow per meaningful gate: `ci.yml` for formatting, lint, unit tests and race checks; `integration.yml` for service integration; `security.yml` for dependency and image scans; `sbom.yml` for CycloneDX output; `release.yml` for versioned artifacts; `provenance.yml` for SLSA attestations and signing; `deploy.yml` for approved GitOps promotion; and `chaos.yml` for scheduled or manually dispatched fault tests. Pin third-party actions to immutable commit SHAs and use least-privilege `permissions`.

### `api/v1alpha1/`

Contains Kubernetes API types, not controller behavior. Define `BackupPolicy`, `Backup`, and `Restore` specs/statuses here, plus group/version registration and generated deepcopy code. CRD schema generation should produce artifacts under `config/crd/bases/`.

### `cmd/`

Each subdirectory is a thin executable composition root. `main.go` should parse configuration, construct dependencies, start health/metrics endpoints, and handle shutdown. Domain logic belongs in `internal/`, not in command packages.

- `aegis-operator/`: controller-runtime manager and controller registration.
- `aegis-worker/`: task execution and streaming data pipeline process.
- `aegis-metadata/`: Raft node process and metadata gRPC server.
- `aegis-storage/`: shard storage gRPC server and local/S3-backed adapter selection.
- `aegis-cli/`: operator-facing inspection and administrative commands; destructive retention actions must use the approval API.

### `config/`

Source configuration for Kubernetes API, permissions, security policy, and component deployment. Generated CRDs belong under `crd/bases`; RBAC manifests should be minimal and reviewed with the corresponding controller code. `helm/` is the maintained chart source. `manifests/` contains Kustomize roots or shared resources. Avoid duplicating deployable manifests between `config/` and `deploy/`; use one canonical source and package or reference it from the other.

### `deploy/`

Environment-specific delivery assets. `k8s/` holds overlays and cluster-specific composition, `helm/aegis/` may hold packaged chart metadata if the chart source is maintained in `config/helm/aegis/`, and `argocd/` declares GitOps applications. Keep credentials and cluster-specific secrets out of Git; use external secret management.

### `docs/`

Project documentation. Keep the implementation plan and this structure guide here. `runbooks/` is for actionable operational procedures such as backup failure triage, restore drills, key rotation, Raft recovery, shard repair, upgrades, and full-cluster disaster recovery. Architecture decision records can be added here as `adr/` when a decision needs a durable rationale.

### `internal/`

Private implementation packages. Package tests should normally live beside the implementation as `*_test.go`; `tests/` is reserved for cross-package/system tests.

- `blobstore/`: define object semantics and local/S3 adapters. A shard/object write must be atomic from the caller's perspective, checksum-validated, and context-cancellable.
- `backup/`: coordinate backup workflow stages and persist resumable progress.
- `chunker/`: stream bytes into bounded FastCDC chunks; avoid buffering an entire volume.
- `compression/`: define compression interfaces and codec wrappers; keep codecs independently testable.
- `crypto/`: SHA-256, zstd, AES-256-GCM, and manifest signing/verifying. Keep key-provider calls behind interfaces and never persist plaintext key material.
- `dedup/`: content identity lookup and reference accounting. The durable authoritative index belongs in metadata; package-local caches are advisory only.
- `erasure/`: Reed-Solomon (6,3) codec, framing, shard validation, and reconstruction.
- `health/`: expose liveness and readiness HTTP endpoints shared by service binaries.
- `metadata/`: Raft integration, deterministic state machine commands, persistent FSM snapshots, linearizable read barriers, and metadata gRPC service implementation.
- `operator/`: policy controller, backup/restore reconciliation, scheduler, and worker assignment. Reconcile handlers must be idempotent and resumable.
- `restore/`: verify signed manifest first, fetch/reconstruct shards, decrypt/decompress/hash-check, write to target, and report readiness/RTO. Fail closed on any integrity error.
- `scheduler/`: schedule durable tasks and assign work to available workers.
- `storage/`: shard server, placement policy, shard readers/writers, and repair workflows.
- `service/`: shared HTTP server startup, structured logging, and graceful shutdown for service entry points.
- `task/`: durable task state, leases, heartbeat, idempotency keys, attempts, and resume checkpoints.
- `security/`: mTLS/SPIFFE identity checks, Kubernetes/OIDC authz, dual approval, tenant isolation, and tamper-evident audit records.
- `kms/`: provider-neutral KMS interface and AWS KMS/Vault Transit adapters; tests should use a deterministic test provider but production code must not fall back to local plaintext keys.
- `gc/`: mark/tombstone/grace-period/sweep process coordinated with metadata and active tasks; deletion must be rechecked atomically before physical removal.
- `recovery/`: resumable backup/restore logic, shard repair orchestration, cold catalogue import/export, and disaster recovery.
- `observability/`: metric registration, trace propagation, structured logging fields, and dashboard/alert assets or generation helpers.

### `pkg/`

Packages intended to be imported by other AEGIS modules or external clients. Keep this surface small and stable.

- `client/`: typed client wrappers for gRPC and Kubernetes APIs.
- `types/`: stable shared records such as manifest, task, status, and domain errors. Avoid duplicating protobuf-generated message definitions.
- `util/`: narrowly scoped helpers used by multiple packages; avoid turning this into a general-purpose dumping ground.
- `proto/generated/`: generated protobuf/gRPC Go sources produced from `proto/`; regeneration should be reproducible through `Makefile` or `scripts/`.

### `proto/`

Versioned protobuf source contracts. Use package versions such as `aegis.storage.v1` and `aegis.metadata.v1`; keep request IDs/idempotency tokens on mutating calls, define explicit error semantics, and reserve removed field numbers/names. Generated Go output goes to `pkg/proto/generated/` and is never edited manually.

### `scripts/`

Small, repeatable automation invoked by developers and CI: code generation, local environment setup, lint/test wrappers, SBOM creation, signing, and benchmark capture. Scripts should fail on errors and avoid embedding secrets.

### `tests/`

System-level tests that need multiple packages or infrastructure.

- `unit/`: only shared test helpers or broad unit suites; package-local tests belong beside production files.
- `integration/`: real local blobstore, gRPC, Raft, KMS test provider, and full data-plane composition.
- `e2e/`: kind or equivalent Kubernetes cluster tests, including the PostgreSQL backup/destroy/restore/row-equality demonstration.
- `chaos/`: repeatable worker/node/storage/KMS/S3/network fault scenarios and success assertions.
- `fuzz/`: Go fuzz targets and corpus data; ensure fuzz tests do not require production credentials.
- `linearizability/`: operation history capture and Porcupine model checks for metadata operations.
- `benchmarks/`: reproducible benchmarks with dataset description, machine details, command, and recorded results.

## Initial implementation conventions

1. Keep executable entry points thin and place business logic under `internal/`.
2. Put unit tests next to the package they exercise; use `tests/` for cross-package and infrastructure tests.
3. Define interfaces at the consumer boundary and make all long-running operations context-aware.
4. Treat generated CRDs and protobuf code as reproducible build outputs.
5. Do not store credentials, plaintext DEKs/KEKs, production data, or local object-store contents in the repository.
6. Before introducing a second source directory for the same deployment artifact, document which is canonical and make the other generated or packaged.
