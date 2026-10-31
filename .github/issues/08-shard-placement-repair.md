# Issue #8: Shard Placement & Repair Workflow

**Epic**: Data Plane | **Area**: Data Plane | **Priority**: P0

## Description

Implement deterministic shard placement across storage nodes and automated repair workflow for failed/missing shards. This ensures data durability and availability.

## Scope

### Placement Algorithm

- **Consistent hashing** with virtual nodes for uniform distribution
- **Replication factor**: 9 shards across N storage nodes (N >= 3)
- **Placement constraints**:
  - No two shards of same chunk on same node
  - Rack/zone awareness (prefer different failure domains)
  - Capacity-aware (don't overload full nodes)

### Shard Placement Interface

```go
// internal/placement/placement.go
type Placer interface {
    // Place returns node assignments for 9 shards
    // Input: chunk hash, available nodes with capacities
    // Output: [9]NodeAssignment (shard_index -> node_id)
    Place(chunkHash [32]byte, nodes []NodeInfo) ([9]NodeAssignment, error)
    
    // Rebalance suggests moves for cluster changes
    Rebalance(nodes []NodeInfo) ([]Move, error)
}

type NodeInfo struct {
    ID       string
    Address  string
    Capacity int64
    Used     int64
    Zone     string
    Rack     string
    Healthy  bool
}

type NodeAssignment struct {
    ShardIndex int
    NodeID     string
    NodeAddr   string
}
```

### Repair Workflow

```
┌─────────────────────────────────────────────────────────────┐
│                    REPAIR CONTROLLER                        │
├─────────────────────────────────────────────────────────────┤
│  1. Detect missing shards (periodic scan + metadata)       │
│  2. For each missing shard:                                 │
│     a. Find 6 healthy shards of same chunk                 │
│     b. Download from 6 nodes                                │
│     c. Decode to reconstruct chunk                          │
│     d. Re-encode to get missing shard                       │
│     e. Place on healthy node (per placement algorithm)     │
│     f. Update metadata with new location                    │
│  3. Track repair progress, retry on failure                │
│  4. Alert if repair stalls or insufficient healthy shards  │
└─────────────────────────────────────────────────────────────┘
```

### Repair Interface

```go
// internal/repair/repair.go
type RepairController interface {
    // ScanAndRepair finds and repairs missing shards
    ScanAndRepair(ctx context.Context) (RepairReport, error)
    
    // RepairChunk repairs a specific chunk
    RepairChunk(ctx context.Context, chunkHash [32]byte) error
    
    // GetRepairStatus returns ongoing repair status
    GetRepairStatus() RepairStatus
}

type RepairReport struct {
    ScannedChunks    int
    MissingShards    int
    RepairedShards   int
    FailedRepairs    int
    Duration         time.Duration
}
```

### Files to Create

- `internal/placement/consistent_hash.go` - Consistent hashing with virtual nodes
- `internal/placement/placer.go` - Placement algorithm
- `internal/placement/placer_test.go` - Tests
- `internal/repair/controller.go` - Repair controller
- `internal/repair/controller_test.go` - Tests
- `internal/repair/metrics.go` - Repair metrics
- `cmd/aegis-repair/main.go` - Repair daemon entry point

## Acceptance Criteria

- [ ] Consistent hashing distributes shards uniformly
- [ ] Placement respects constraints (no same-node duplicates, zone awareness)
- [ ] Rebalance minimizes data movement on node add/remove
- [ ] Repair controller detects missing shards from metadata
- [ ] Repair reconstructs from 6 healthy shards
- [ ] Repaired shard placed on appropriate node
- [ ] Metadata updated with new shard location
- [ ] Repair metrics: scanned, repaired, failed, duration
- [ ] Concurrent repairs limited (configurable)
- [ ] Repair daemon runs continuously with backoff

## Test Cases

1. **Placement uniform** - 1000 chunks across 5 nodes → ~even distribution
2. **Constraints** - No chunk has 2 shards on same node
3. **Zone awareness** - Shards spread across zones when possible
4. **Rebalance** - Add node → minimal data movement
5. **Repair single** - 1 missing shard → reconstructed and placed
6. **Repair multiple** - Multiple chunks missing → parallel repair
6. **Insufficient shards** - Only 5 healthy → repair fails with alert
7. **Concurrent repairs** - Limit concurrent operations
8. **Node failure** - Simulate node down → repair triggered
9. **Metrics** - Repair metrics accurate

## Dependencies

- Requires: #1 (Scaffolding), #6 (Erasure Coding), #7 (Storage Node), #9 (Metadata)
- Blocks: #13 (Backup Workflow), #14 (Restore Workflow), #18 (Chaos Testing)

## Definition of Done

- All tests pass
- Repair daemon integrates with metadata plane
- Chaos testing validates repair under failures
- Documentation in `internal/placement/README.md` and `internal/repair/README.md`

## References

- [Implementation Plan - Task 9](../docs/implementation-plan.md#task-9---implement-the-shard-placement-and-repair-workflow)
- Consistent Hashing: https://en.wikipedia.org/wiki/Consistent_hashing
- Rendezvous Hashing alternative