# Issue #14: Restore Workflow Implementation

**Epic**: Workflows | **Area**: Restore, Operator | **Priority**: P0

## Description

Implement the end-to-end restore workflow: locate manifest, verify signature, fetch shards, reconstruct chunks, decrypt, decompress, verify hashes, stream to PVC, recreate workload objects.

## Scope

### Restore Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    RESTORE WORKFLOW                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. LOCATE                                                      │
│     ├── Get Restore CR → snapshotRef                           │
│     ├── Fetch manifest from metadata                           │
│     └── Verify manifest exists                                 │
│                                                                 │
│  2. VERIFY                                                      │
│     ├── Verify manifest signature (operator public key)        │
│     ├── Verify metadata commit (Raft quorum)                   │
│     ├── Check retention lock (not expired, not deleted)        │
│     └── Fail fast on any mismatch                              │
│                                                                 │
│  3. FETCH & RECONSTRUCT                                         │
│     ├── For each chunk in manifest:                            │
│     │   ├── Get shard locations from metadata                  │
│     │   ├── Fetch 6+ shards from storage nodes (parallel)     │
│     │   ├── Reed-Solomon decode → encrypted chunk             │
│     │   ├── Unwrap DEK from KMS                                │
│     │   ├── AES-256-GCM decrypt                                │
│     │   ├── zstd decompress                                    │
│     │   ├── Verify SHA-256 matches manifest                    │
│     │   └── Stream to target PVC                               │
│     └── Handle missing/corrupt shards → repair or fail        │
│                                                                 │
│  4. RECREATE WORKLOAD                                           │
│     ├── Create/restore PVC with data                           │
│     ├── Recreate Kubernetes objects from manifest              │
│     ├── Wait for readiness                                     │
│                                                                 │
│  5. RTO MEASUREMENT                                             │
│     ├── Record: request created, manifest verified             │
│     ├── Record: metadata fetched, first byte restored          │
│     ├── Record: readiness achieved, total duration             │
│     └── Export metrics                                         │
│                                                                 │
│  6. COMPLETE                                                    │
│     ├── Update Restore CR status                               │
│     ├── Emit completion event                                  │
│     └── Release task lease                                     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Restore Handler

```go
// internal/worker/restore_handler.go
type RestoreHandler struct {
    metadata    metadata.MetadataStore
    storage     storage.ShardStore
    erasure     erasure.Encoder
    kms         crypto.KMSProvider
    decryptor   crypto.Decryptor
    decompressor compression.Compressor
    verifier    crypto.Verifier
    k8sClient   kubernetes.Interface
}

func (h *RestoreHandler) Execute(ctx context.Context, task Task) error {
    // Implement full flow above
}
```

### Integrity Guarantees

- **Never return corrupted data**: Any hash mismatch → immediate failure
- **Fail explicit**: Clear error messages for each failure mode
- **Atomic**: Partial restore leaves no orphaned resources

### Files to Create

- `internal/worker/restore_handler.go` - Main restore execution
- `internal/worker/restore_handler_test.go` - Unit tests
- `internal/worker/stream_writer.go` - Streaming to PVC
- `internal/worker/workload_recreator.go` - K8s object recreation
- `internal/worker/rto_recorder.go` - RTO measurement
- `test/integration/restore_test.go` - End-to-end restore test

## Acceptance Criteria

- [ ] Full restore flow executes successfully
- [ ] Manifest signature verified before any data fetch
- [ ] Metadata commit verified (quorum)
- [ ] Shards fetched in parallel from multiple nodes
- [ ] Reed-Solomon decode with 6 of 9 shards
- [ ] DEK unwrapped from KMS
- [ ] Decrypt → decompress → hash verify pipeline
- [ ] Streaming write to target PVC
- [ ] Kubernetes objects recreated from manifest
- [ ] Readiness checks pass
- [ ] RTO metrics recorded and exported
- [ ] Any integrity failure → explicit error, no partial data

## Test Cases

1. **Simple restore** - Single PVC, all shards available
2. **Missing shards** - 3 shards missing → reconstruct from 6
3. **Corrupt shard** - 1 shard corrupted → detect via checksum
4. **KMS failure** - DEK unwrap fails → clear error
5. **Hash mismatch** - Decrypted data hash ≠ manifest → fail
6. **Large restore** - 100GB, streaming works
7. **Partial failure** - One chunk fails → entire restore fails
8. **RTO measurement** - Metrics match expected phases
9. **Workload recreation** - K8s objects match original
10. **Concurrent restores** - Multiple restores, isolated

## Dependencies

- Requires: #3, #4, #5, #6, #7, #9, #10, #12, #13
- Blocks: #15, #17, #18, #19

## Definition of Done

- End-to-end restore test passes in CI
- Integrity verification: corrupted data never returned
- RTO metrics accurate
- Documentation in `internal/worker/README.md`

## References

- [Implementation Plan - Task 14](../docs/implementation-plan.md#task-14---implement-restore-path)