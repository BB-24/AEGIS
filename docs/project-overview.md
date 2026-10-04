# AEGIS Project Overview

This repository contains the implementation foundation for AEGIS, a secure, distributed, cloud-native data storage and recovery platform built for resilience, integrity, and operational safety.

## Project Goal

AEGIS is designed to provide:

- durable object storage with content-defined chunking
- strong integrity guarantees using cryptographic hashes and checksums
- erasure-coded recovery for data loss tolerance
- distributed metadata coordination with consensus-backed state
- secure workload identity and policy enforcement
- Kubernetes-native deployment and GitOps-friendly operations

## Core Project Areas

### 1. Data Plane
The data plane is responsible for storing, encrypting, chunking, and reconstructing data.

- `internal/blobstore/` - object storage abstractions and backing-store logic
- `internal/chunker/` - content-defined chunking, deduplication inputs, and chunk boundaries
- `internal/crypto/` - hashing, encryption, signing, and integrity logic
- `internal/erasure/` - Reed-Solomon and recovery logic for resilient data reconstruction
- `internal/dedup/` - duplicate detection and content identity workflows
- `internal/recovery/` - restore and repair operations after corruption or loss
- `internal/restore/` - recovery orchestration for returning data to usable state
- `internal/storage/` - storage runtime plumbing and data lifecycle orchestration

### 2. Metadata and Control Plane
This layer tracks object state, metadata consistency, and cluster coordination.

- `internal/metadata/` - metadata indexing, object tracking, and state management
- `internal/operator/` - Kubernetes operator logic for CRDs and lifecycle management
- `api/v1alpha1/` - API types and custom resource definitions for cluster interactions
- `config/crd/bases/` - generated and checked-in CRD manifests
- `config/rbac/` - permissions and access policy definitions
- `cmd/aegis-operator/` - operator binary entry point
- `cmd/aegis-metadata/` - metadata service binary entry point

### 3. Workload and Service Runtime
These components perform the execution work needed for chunk processing, tasks, and orchestration.

- `internal/task/` - background job execution and task lifecycle management
- `internal/gc/` - garbage collection and retention cleanup processing
- `cmd/aegis-worker/` - worker service entry point for processing workloads
- `cmd/aegis-storage/` - storage service entry point
- `cmd/aegis-cli/` - command-line interface for operators and administrators

### 4. Security and Trust
Security is a first-class concern across the platform and includes workload trust, enforcement, and provenance.

- `internal/security/` - security policies and enforcement routines
- `config/security/kyverno/` - policy validation for Kubernetes workloads
- `config/security/opa/` - policy-as-code enforcement and authorization logic
- `config/security/spire/` - SPIFFE/SPIRE identity and workload trust configuration
- `internal/kms/` - key management and encryption key orchestration

### 5. Interface and API Contracts
The platform uses protocol contracts to connect components and standardize service behavior.

- `proto/` - protocol buffer definitions for service APIs
- `pkg/proto/generated/` - generated Go bindings and service stubs
- `pkg/client/` - client libraries and SDK-style wrappers for service interaction
- `pkg/types/` - shared domain types and metadata structures
- `pkg/util/` - reusable utilities and helpers

### 6. Deployment and Operations
This area covers packaging, deployment automation, and production operations.

- `deploy/k8s/` - Kubernetes manifests and deployment assets
- `deploy/helm/aegis/` - Helm chart for platform deployment
- `deploy/argocd/` - GitOps and Argo CD integration manifests
- `config/helm/aegis/` - chart configuration and reusable deployment resources
- `config/manifests/` - static manifest bundles and packaging manifests
- `scripts/` - setup, automation, and operational scripts
- `helm/aegis/` - Helm templates and chart assets for distribution

### 7. Testing and Validation
Reliability and correctness are protected through layered testing and simulated failure modes.

- `tests/unit/` - unit test suite for component correctness
- `tests/integration/` - cross-component integration checks
- `tests/e2e/` - end-to-end system behavior validation
- `tests/chaos/` - fault injection and failure-mode testing
- `tests/linearizability/` - concurrency and consistency validation
- `tests/benchmarks/` - performance and throughput measurements
- `tests/fuzz/` - fuzz and robustness testing

## Architectural Direction

The repository is organized around a modular service architecture where each subsystem has a clear responsibility:

- storage and recovery handle the actual data lifecycle
- metadata and the operator maintain state and cluster orchestration
- security components protect identity, policy, and encryption flows
- deployment manifests enable Kubernetes-based rollout and GitOps adoption
- tests validate correctness under normal, degraded, and adversarial conditions

## Recommended Reading Order

If you are onboarding to the project, the best sequence is:

1. `docs/implementation-plan.md` - overall engineering strategy and roadmap
2. `docs/repository-structure.md` - repository layout and folder responsibilities
3. `api/v1alpha1/` and `proto/` - API contracts and object schemas
4. `internal/` packages - runtime implementation details
5. `deploy/` and `config/` - installation and cluster configuration
6. `tests/` - validation approach and expected behavior

## Summary

AEGIS is intended to be a secure, resilient, scalable data platform whose foundations are split cleanly across storage, metadata, security, orchestration, and deployment concerns. The repository structure is organized to support incremental implementation and long-term engineering execution.
