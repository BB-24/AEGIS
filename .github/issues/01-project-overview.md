# AEGIS Project - GitHub Issues

This directory contains GitHub issue templates covering the full scope of the AEGIS project as defined in the [Implementation Plan](../docs/implementation-plan.md).

## Project Summary

**AEGIS: Immutable Distributed Data Protection Platform for Kubernetes**

A zero-trust backup and recovery platform designed for stateful workloads in Kubernetes. It protects against ransomware, administrative compromise, cluster takeover, storage corruption, and data loss while preserving an immutable, auditable backup lineage.

### Four Trust Anchors

1. **Data Plane** - Encrypted, content-addressed chunks and erasure-coded shards in distributed storage
2. **Metadata Plane** - Strongly consistent Raft cluster for snapshots, chunk locations, retention locks, task state
3. **Control Plane** - Kubernetes operator and CRDs for backup scheduling, snapshot lifecycle, policy enforcement
4. **Security Plane** - Workload identity, external KMS access, dual approval for destructive actions, signed manifests

---

## Epic Structure

The project is organized into 19 implementation tasks grouped into logical phases:

| Phase | Tasks | Description |
|-------|-------|-------------|
| **Phase 1: Foundation** | 1-3 | Project scaffolding, FastCDC chunking, cryptography primitives |
| **Phase 2: Data Plane** | 4-9 | Content-defined chunking, dedup, compression, encryption, erasure coding, storage nodes, shard placement |
| **Phase 3: Metadata Plane** | 10 | Raft-based metadata state machine |
| **Phase 4: API & Control Plane** | 11-13 | gRPC contracts, Kubernetes operator, scheduler |
| **Phase 5: Workflows** | 13-14 | Backup and restore execution paths |
| **Phase 6: Security** | 15 | SPIFFE/SPIRE, KMS, dual approval, audit logging |
| **Phase 7: Platform** | 16-19 | CI/CD, observability, chaos testing, disaster recovery |