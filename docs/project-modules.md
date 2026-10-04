# Project Modules and Responsibilities

This document describes the major modules in the AEGIS repository and the responsibility of each area.

## 1. API and CRD Layer

### Purpose
Defines the external contract for the platform and the Kubernetes resource model used by operators and controllers.

### Responsibilities
- declare the custom resources and API versions
- define object schemas and configuration models
- provide the contract for operator-managed workloads
- support declarative configuration and compatibility with Kubernetes patterns

### Key locations
- `api/v1alpha1/`
- `config/crd/bases/`
- `config/rbac/`

---

## 2. Command Layer

### Purpose
Provides the executable binaries that run the platform services and tools.

### Responsibilities
- bootstrap and start service processes
- expose entry points for metadata, storage, worker, and operator components
- provide CLI utilities for operations and administration

### Key locations
- `cmd/aegis-cli/`
- `cmd/aegis-metadata/`
- `cmd/aegis-operator/`
- `cmd/aegis-storage/`
- `cmd/aegis-worker/`

---

## 3. Storage Data Plane

### Purpose
Handles raw data movement, storage, and lifecycle operations.

### Responsibilities
- store object data and manage backing storage structures
- chunk large input streams into manageable blocks
- detect duplicates and avoid unnecessary writes
- compress and encrypt stored chunks for safety and confidentiality
- reconstruct data during recovery or restore workflows

### Key locations
- `internal/blobstore/`
- `internal/chunker/`
- `internal/dedup/`
- `internal/storage/`
- `internal/recovery/`
- `internal/restore/`

---

## 4. Integrity and Security Module

### Purpose
Ensures the platform protects data from corruption, tampering, and unauthorized use.

### Responsibilities
- compute and validate hashes for data integrity
- perform encryption and key handling
- protect sensitive metadata and secrets
- enforce security policies and workload trust boundaries
- support identity-based access and mTLS patterns

### Key locations
- `internal/crypto/`
- `internal/kms/`
- `internal/security/`
- `config/security/kyverno/`
- `config/security/opa/`
- `config/security/spire/`

---

## 5. Erasure and Recovery Engine

### Purpose
Provides resilience against partial data loss or hardware failure.

### Responsibilities
- break data into shards for fault tolerance
- encode and decode by using erasure-coding techniques
- rebuild lost or corrupted fragments
- support disaster recovery and repair workflows

### Key locations
- `internal/erasure/`
- `internal/recovery/`
- `internal/restore/`

---

## 6. Metadata and Coordination Layer

### Purpose
Maintains the state of objects, tasks, and cluster coordination.

### Responsibilities
- store metadata and service state
- coordinate distributed consistency decisions
- track data placement and health state
- manage cluster-level orchestration and lifecycle events

### Key locations
- `internal/metadata/`
- `internal/operator/`
- `pkg/types/`
- `pkg/proto/generated/`

---

## 7. Workflow and Task Execution

### Purpose
Runs background jobs and organized operational tasks needed by storage and metadata services.

### Responsibilities
- schedule and execute internal processing jobs
- manage retention, compaction, and cleanup operations
- coordinate async operations across subsystems
- support recoverable and observable task execution

### Key locations
- `internal/task/`
- `internal/gc/`

---

## 8. Protobuf and Shared Contracts

### Purpose
Defines shared API communication between the system components.

### Responsibilities
- specify gRPC service contracts
- define request and response payloads
- maintain versioned interfaces across service boundaries
- provide generated client/server code for internal integrations

### Key locations
- `proto/`
- `pkg/proto/generated/`
- `pkg/client/`

---

## 9. Shared Types and Utilities

### Purpose
Provides common, reusable abstractions for the platform.

### Responsibilities
- define domain models used across modules
- centralize utility logic and common helpers
- reduce duplication across storage, metadata, security, and orchestration code

### Key locations
- `pkg/types/`
- `pkg/util/`

---

## 10. Deployment and Kubernetes Platform

### Purpose
Enables deployment into real Kubernetes environments and GitOps-friendly operations.

### Responsibilities
- package the platform for deployment
- configure Helm charts and manifests
- provide Argo CD and operational deployment patterns
- manage cluster-level rollout, scaling, and observability plumbing

### Key locations
- `deploy/k8s/`
- `deploy/helm/aegis/`
- `deploy/argocd/`
- `config/helm/aegis/`
- `config/manifests/`
- `helm/aegis/`

---

## 11. Observability and Operations

### Purpose
Makes the platform manageable and diagnosable in production.

### Responsibilities
- expose telemetry, logs, and metrics
- support tracing and health monitoring
- provide debugging and operational insights
- help validate cluster health during deployment and recovery

### Key locations
- `internal/observability/`
- `scripts/`
- `docs/runbooks/`

---

## 12. Testing and Validation

### Purpose
Ensures the system behaves correctly under normal, degraded, and adversarial conditions.

### Responsibilities
- verify unit-level correctness
- validate cross-service integration behavior
- simulate failure and resilience scenarios
- measure benchmark performance and throughput
- detect consistency issues and regression risk

### Key locations
- `tests/unit/`
- `tests/integration/`
- `tests/e2e/`
- `tests/chaos/`
- `tests/linearizability/`
- `tests/benchmarks/`
- `tests/fuzz/`

---

## Module Ownership Summary

| Area | Primary Responsibility |
| --- | --- |
| `api/` and `config/` | API contract and Kubernetes resource definition |
| `cmd/` | service entry points and executables |
| `internal/blobstore/` and `internal/storage/` | data storage and storage lifecycle |
| `internal/chunker/` and `internal/dedup/` | data segmentation and duplicate elimination |
| `internal/crypto/` and `internal/kms/` | encryption, integrity, and key handling |
| `internal/erasure/` and `internal/recovery/` | fault tolerance and recovery |
| `internal/metadata/` and `internal/operator/` | state management and orchestration |
| `internal/task/` and `internal/gc/` | operational execution and cleanup |
| `proto/` and `pkg/client/` | service interfaces and client integration |
| `deploy/` and `config/` | cluster deployment and Kubernetes configuration |
| `tests/` | validation, performance, and resilience verification |

## Final Note

The repository is organized by responsibility rather than by feature alone. Each module is intended to have a narrow, clear ownership boundary so that the overall platform remains maintainable, testable, and secure as it evolves.
