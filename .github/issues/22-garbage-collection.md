# Issue #22: Garbage Collection Design & Implementation

**Epic**: Cross-Cutting | **Area**: Data Plane, Storage | **Priority**: P1

## Description

Design and implement fail-safe garbage collection that never deletes referenced or in-flight data. GC must only delete unreferenced chunks after grace period with metadata and blobstore agreement.

## Scope

### Requirements

- GC only deletes chunks unreferenced by any committed snapshot
- Unreferenced chunks kept for grace period (default 7 days)
- Concurrent backups/restores must not race with deletion
- Metadata and blobstore must agree before permanent deletion

### Reference Tracking

Each chunk has:
- `ref_count` in metadata
- Snapshot-level backlinks
- Pending writes list with lease/hold state

Chunk is live when:
- Exists in at least one committed snapshot
- Referenced by restore/repair task
- Within grace period after retention expiry

### Mark-and-Sweep Algorithm

1. Build live set from committed snapshots
2. For each snapshot, walk all chunk hashes
3. Add each hash to live set
4. Enumerate all stored chunks in blobstore
5. Delete only objects not in live set AND older than grace period
6. Before physical deletion, verify metadata still considers it unreferenced

### Grace Periods

- Default: 7 days for unreferenced chunks
- Longer for retention-critical/compliance workloads
- Never delete before metadata engine and repair daemon agree safe

### Pending Writes & In-Flight Tasks

- Mark pending writes with temporary hold before deleting
- If chunk referenced by task, skip GC
- If backup in-progress, defer sweep until completion

### Crashes During GC

- Use tombstone/GC marker in metadata
- If GC interrupted, chunk remains until next cycle
- Don't physically delete until final verification passes

### Race Condition Handling

Key rule: Never physically delete if metadata reports positive ref_count or active task.

### Files to Create

- `internal/gc/controller.go` - GC controller
- `internal/gc/mark_sweep.go` - Mark-and-sweep implementation
- `internal/gc/grace_period.go` - Grace period management
- `internal/gc/tombstone.go` - Tombstone handling
- `internal/gc/metrics.go` - GC metrics
- `cmd/aegis-gc/main.go` - GC daemon entry point
- `test/integration/gc_test.go` - GC integration tests

## Acceptance Criteria

- [ ] Mark-and-sweep correctly identifies live chunks
- [ ] Grace period enforced before deletion
- [ ] Concurrent backups/restores protected
- [ ] Metadata and blobstore agree before delete
- [ ] Crash during GC leaves data intact
- [ ] Tombstones cleaned up after verification
- [ ] Metrics: candidates, deleted, errors, duration
- [ ] GC runs on schedule (configurable)

## Test Cases

1. **Basic GC** - Unreferenced chunks deleted after grace period
2. **Referenced chunks** - Live chunks never deleted
3. **Concurrent backup** - In-flight writes protected
4. **Concurrent restore** - Restore refs protected
5. **Crash during GC** - Data intact, tombstones present
6. **Grace period** - Chunks not deleted before expiry
7. **Metadata-blobstore sync** - Both must agree
8. **Repair daemon** - Repair refs protected
9. **Large scale** - 1M chunks, GC completes in reasonable time
10. **Metrics accuracy** - Counters match actual deletions

## Dependencies

- Requires: #4 (Dedup), #7 (Storage), #9 (Metadata)
- Blocks: #13, #14, #19

## Definition of Done

- GC daemon integrated with metadata and storage
- Chaos testing validates no data loss
- Runbook for GC operations
- Documentation in `internal/gc/README.md`

## References

- [Implementation Plan - Section 13](../docs/implementation-plan.md#13-garbage-collection-design)