# AEGIS GitHub Issues

This directory contains the complete set of GitHub issues for the AEGIS project, derived from the [Implementation Plan](../docs/implementation-plan.md).

## Issue Summary

| # | Title | Epic | Area | Priority |
|---|-------|------|------|----------|
| 1 | [Project Scaffolding & Build System](01-project-scaffolding.md) | Foundation | Build | P0 |
| 2 | [FastCDC Content-Defined Chunking](02-fastcdc-chunking.md) | Foundation | Data Plane | P0 |
| 3 | [AES-256-GCM Cryptography Primitives](03-cryptography-primitives.md) | Foundation | Security, Data Plane | P0 |
| 4 | [Chunk Deduplication & zstd Compression](04-chunk-dedup-compression.md) | Data Plane | Data Plane | P0 |
| 5 | [Envelope Encryption with KMS Integration](05-envelope-encryption-kms.md) | Data Plane | Security, Data Plane | P0 |
| 6 | [Reed-Solomon (6,3) Erasure Coding](06-reed-solomon-erasure-coding.md) | Data Plane | Data Plane | P0 |
| 7 | [Storage Node gRPC Server & Shard Persistence](07-storage-node-grpc.md) | Data Plane | Data Plane, gRPC | P0 |
| 8 | [Shard Placement & Repair Workflow](08-shard-placement-repair.md) | Data Plane | Data Plane | P0 |
| 9 | [Raft Metadata State Machine](09-raft-metadata-state-machine.md) | Metadata Plane | Metadata, Distributed Systems | P0 |
| 10 | [gRPC Service Contracts & Go SDK](10-api-grpc-contracts.md) | Control Plane | API, gRPC | P0 |
| 11 | [Kubernetes Operator & CRDs](11-kubernetes-operator.md) | Control Plane | Operator, Kubernetes | P0 |
| 12 | [Scheduler & Task Manager](12-scheduler-task-manager.md) | Control Plane | Operator, Scheduling | P0 |
| 13 | [Backup Workflow Implementation](13-backup-workflow.md) | Workflows | Backup, Operator | P0 |
| 14 | [Restore Workflow Implementation](14-restore-workflow.md) | Workflows | Restore, Operator | P0 |
| 15 | [Security Controls: SPIFFE, KMS, Dual Approval, Audit](15-security-controls.md) | Security | Security, Identity | P0 |
| 16 | [Secure CI/CD Pipeline & Supply Chain](16-ci-cd-supply-chain.md) | Platform | CI/CD, Supply Chain | P1 |
| 17 | [Observability: Metrics, Tracing, Logging, Dashboards](17-observability.md) | Platform | Observability | P1 |
| 18 | [Chaos Testing & Fault Injection](18-chaos-testing.md) | Platform | Testing, Chaos | P1 |
| 19 | [Cold Storage & Disaster Recovery Procedures](19-disaster-recovery.md) | Platform | DR, Backup | P1 |
| 20 | [Data Model & API Design Specification](20-data-model-api-design.md) | Cross-Cutting | API, Design | P0 |
| 21 | [Distributed Systems Design](21-distributed-systems-design.md) | Cross-Cutting | Distributed Systems, Design | P0 |
| 22 | [Garbage Collection Design & Implementation](22-garbage-collection.md) | Cross-Cutting | Data Plane, Storage | P1 |
| 23 | [Security Engineering & Threat Model](23-security-threat-model.md) | Cross-Cutting | Security, Design | P0 |
| 24 | [Restore Verification & Integrity Checks](24-restore-verification.md) | Cross-Cutting | Restore, Security | P0 |
| 25 | [Performance Benchmarks & SLA Targets](25-performance-benchmarks.md) | Cross-Cutting | Performance, Testing | P2 |
| 26 | [Documentation: Architecture, API, Operations](26-documentation.md) | Cross-Cutting | Documentation | P2 |

## Phase Breakdown

### Phase 1: Foundation (Month 1-2)
Issues: #1, #2, #3, #20, #23

### Phase 2: Data Plane Core (Month 3-5)
Issues: #4, #5, #6, #7, #8, #21, #22

### Phase 3: Metadata Plane (Month 6)
Issues: #9, #10

### Phase 4: Control Plane (Month 7-8)
Issues: #11, #12, #24

### Phase 5: Workflows (Month 9-10)
Issues: #13, #14

### Phase 6: Security Hardening (Month 11)
Issue: #15

### Phase 7: Platform & Release (Month 12-13)
Issues: #16, #17, #18, #19, #25, #26

## Creating Issues in GitHub

Use GitHub CLI:
```bash
for f in .github/issues/*.md; do
  gh issue create --title "$(head -1 "$f" | sed 's/# Issue #//')" --body-file "$f" --label "$(grep 'Labels:' "$f" | sed 's/.*Labels: //')"
done
```

## Labels Reference

### Epic Labels
- `epic:foundation` - Core infrastructure and primitives
- `epic:data-plane` - Storage, chunking, encryption, erasure coding
- `epic:metadata-plane` - Raft metadata cluster
- `epic:control-plane` - Operator, scheduler, API
- `epic:workflows` - Backup and restore execution
- `epic:security` - Identity, encryption, approval, audit
- `epic:platform` - CI/CD, observability, chaos, DR

### Priority Labels
- `P0` - Critical path, blocks other work
- `P1` - High priority, needed for MVP
- `P2` - Medium priority, post-MVP
- `P3` - Low priority, nice to have

---

*Generated from [implementation-plan.md](../docs/implementation-plan.md)*