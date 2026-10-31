# Issue #21: Distributed Systems Design - Raft, Quorum, Consistency

**Epic**: Cross-Cutting | **Area**: Distributed Systems, Design | **Priority**: P0

## Description

Document the distributed systems design decisions for AEGIS: Raft consensus, quorum configuration, linearizability guarantees, state machine replication, snapshots, membership changes, network partition handling, and retry semantics.

## Scope

### Raft Configuration

- **Cluster size**: 3 nodes (dev), 5 nodes (production)
- **Election timeout**: 150-300ms (randomized)
- **Heartbeat interval**: 50ms
- **Snapshot interval**: 30s or 10k entries
- **Max append entries**: 64

### Quorum Semantics

- **Write quorum**: Majority (2/3 or 3/5)
- **Read quorum**: ReadIndex for linearizable reads
- **Stale reads**: Allowed for non-critical paths (lease reads)

### Linearizability

Critical operations requiring linearizability:
- Chunk existence checks (deduplication)
- Snapshot visibility for restore
- Retention lock queries
- Task lease state

Implementation: Leader completes ReadIndex barrier before serving read.

### State Machine

Deterministic, idempotent state transitions for:
- Chunk index (PutChunk, IncRef, DecRef)
- Snapshot metadata (PutSnapshot, UpdateStatus)
- Task leases (Acquire, Renew, Release)
- Manifests (PutManifest)
- Retention locks (AcquireLock, ReleaseLock)
- Audit log (Append)

### Snapshots & Membership

- Periodic snapshots compact log
- Snapshots include metadata + chunk refs
- Rejoined nodes restore from snapshot + logs
- Member changes use joint consensus

### Network Partitions

- Majority partition: continues serving writes
- Minority partition: rejects writes, serves stale reads (if enabled)
- Clients retry against active leader
- Data safe: restore only uses quorum-committed metadata

### Retry Semantics

- Idempotency keys on all API requests
- Content-addressed chunk IDs (natural dedup)
- Idempotent task lease acquisition
- Deduplication references in metadata

### Files to Create

- `docs/architecture/distributed-systems.md` - Complete design doc
- `docs/architecture/raft-config.md` - Raft tuning parameters
- `docs/architecture/consistency-model.md` - Consistency guarantees
- `internal/metadata/raft_config.go` - Raft configuration constants

## Acceptance Criteria

- [ ] Raft configuration documented with rationale
- [ ] Quorum semantics defined for all operations
- [ ] Linearizability requirements enumerated
- [ ] State machine commands specified
- [ ] Partition behavior documented
- [ ] Retry/idempotency patterns established
- [ ] Jepsen test plan defined

## Dependencies

- Requires: #1, #9
- Blocks: #9, #11, #12, #13, #14, #22

## Definition of Done

- Design doc reviewed and approved
- Configuration constants in code match doc
- Test plan for partition tolerance

## References

- [Implementation Plan - Section 12](../docs/implementation-plan.md#12-distributed-systems-design)
- Raft paper: https://raft.github.io/raft.pdf