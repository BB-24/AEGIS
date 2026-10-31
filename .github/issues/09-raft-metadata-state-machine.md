# Issue #9: Raft Metadata State Machine

**Epic**: Metadata Plane | **Area**: Metadata, Distributed Systems | **Priority**: P0

## Description

Implement the strongly consistent metadata plane using Raft consensus. This cluster stores snapshot metadata, chunk index, shard locations, retention locks, task state, and audit log.

## Scope

### Raft Cluster

- **Nodes**: 3 (dev) or 5 (production)
- **Log replication**: Quorum writes
- **Leader election**: Standard Raft
- **ReadIndex**: Linearizable reads
- **Snapshots**: Periodic log compaction
- **Membership changes**: Joint consensus

### State Machine

The replicated state machine maintains:

1. **Chunk Index** - hash to ChunkMetadata
2. **Snapshot Metadata** - snapshot_id to Snapshot
3. **Task Lease State** - task_id to Lease
4. **Manifest Set** - snapshot_id to Manifest
5. **Retention Locks** - snapshot_id to Lock
6. **Audit Log** - append-only event log

## Interface

```go
type MetadataStore interface {
    // Chunk operations
    PutChunk(ctx context.Context, meta ChunkMetadata) error
    GetChunk(ctx context.Context, hash [32]byte) (*ChunkMetadata, error)
    ListChunks(ctx context.Context, prefix [32]byte, limit int) ([]ChunkMetadata, error)
    IncRef(ctx context.Context, hash [32]byte) error
    DecRef(ctx context.Context, hash [32]byte) (bool, error)
    
    // Snapshot operations
    PutSnapshot(ctx context.Context, snap Snapshot) error
    GetSnapshot(ctx context.Context, id string) (*Snapshot, error)
    ListSnapshots(ctx context.Context, filter SnapshotFilter) ([]Snapshot, error)
    UpdateSnapshotStatus(ctx context.Context, id string, status SnapshotStatus) error
    
    // Task lease operations
    AcquireLease(ctx context.Context, taskID, workerID string, ttl time.Duration) (bool, error)
    RenewLease(ctx context.Context, taskID, workerID string) error
    ReleaseLease(ctx context.Context, taskID, workerID string) error
    GetLease(ctx context.Context, taskID string) (*Lease, error)
    
    // Manifest operations
    PutManifest(ctx context.Context, manifest Manifest) error
    GetManifest(ctx context.Context, snapshotID string) (*Manifest, error)
    
    // Retention lock operations
    AcquireLock(ctx context.Context, snapshotID string, lock RetentionLock) error
    ReleaseLock(ctx context.Context, snapshotID string, approval Approval) error
    GetLock(ctx context.Context, snapshotID string) (*RetentionLock, error)
    
    // Linearizable read
    ReadIndex(ctx context.Context) error
    
    // Cluster management
    AddNode(ctx context.Context, nodeID, address string) error
    RemoveNode(ctx context.Context, nodeID string) error
    GetClusterStatus() ClusterStatus
}
```

## Implementation

- **Library**: `github.com/hashicorp/raft` or `github.com/etcd-io/raft`
- **Storage**: BoltDB or BadgerDB for log + snapshots
- **Transport**: gRPC with mTLS
- **Serialization**: Protocol Buffers for state machine commands

### Files to Create

- `proto/metadata/v1/metadata.proto` - State machine commands/events
- `internal/metadata/raft.go` - Raft node implementation
- `internal/metadata/fsm.go` - Finite state machine
- `internal/metadata/store.go` - MetadataStore interface implementation
- `internal/metadata/snapshot.go` - Log compaction/snapshots
- `internal/membership/manager.go` - Cluster membership changes
- `cmd/aegis-metadata/main.go` - Metadata node entry point
- `internal/metadata/*_test.go` - Unit + integration tests
- `test/raft/cluster_test.go` - Multi-node cluster tests

## Acceptance Criteria

- [ ] 3-node cluster forms and elects leader
- [ ] Writes require quorum (2/3 or 3/5)
- [ ] Linearizable reads via ReadIndex
- [ ] State machine applies all command types correctly
- [ ] Snapshots compact log periodically
- [ ] Membership changes work (add/remove nodes)
- [ ] Network partition: minority rejects writes
- [ ] Node restart recovers from snapshot + logs
- [ ] Metrics: commit latency, replication lag, leadership changes
- [ ] Jepsen-style tests pass (Porcupine verification)

## Configuration

```yaml
metadata:
  node_id: "meta-1"
  address: "0.0.0.0:9002"
  cluster:
    - id: "meta-1"
      address: "meta-1:9002"
    - id: "meta-2"
      address: "meta-2:9002"
    - id: "meta-3"
      address: "meta-3:9002"
  raft:
    election_timeout: "150ms"
    heartbeat_interval: "50ms"
    snapshot_interval: "30s"
    snapshot_threshold: 10000
    max_append_entries: 64
  storage:
    path: "/var/lib/aegis/metadata"
```

## Test Cases

1. **Basic cluster** - 3 nodes, leader elected, write/read works
2. **Quorum writes** - Write with 2/3 nodes up, fails with 1/3
3. **Leader failover** - Kill leader, new election, writes continue
4. **Linearizable reads** - ReadIndex prevents stale reads
5. **Log compaction** - Snapshots created, old logs truncated
6. **Membership change** - Add 4th node, joint consensus, remove
7. **Partition tolerance** - Minority partition rejects writes
8. **Node restart** - Restart node, catches up from snapshot
9. **Concurrent operations** - Multiple clients, mixed read/write
10. **Porcupine** - Linearizability verification under partitions
11. **Performance** - Commit latency < 10ms (local), throughput > 10k ops/s

## Dependencies

- Requires: #1 (Scaffolding), #10 (gRPC Contracts)
- Blocks: #11 (Operator), #12 (Scheduler), #13 (Backup), #14 (Restore), #22 (GC)

## Definition of Done

- All tests pass including Porcupine linearizability tests
- Cluster survives network partitions correctly
- Documentation in `internal/metadata/README.md`
- Operations runbook for metadata cluster

## References

- [Implementation Plan - Task 9](../docs/implementation-plan.md#task-9---implement-consistent-metadata-state-machine)
- Raft paper: https://raft.github.io/raft.pdf
- HashiCorp Raft: https://github.com/hashicorp/raft
- Porcupine: https://github.com/anishathalye/porcupine