# Issue #13: Backup Workflow Implementation

**Epic**: Workflows | **Area**: Backup, Operator | **Priority**: P0

## Description

Implement the end-to-end backup workflow: resolve target volume, run pre-backup hook, snapshot, chunk, encrypt, store shards, commit metadata, sign manifest, apply retention.

## Scope

### Backup Flow

1. **RESOLVE** - Get PVC, validate, create CSI VolumeSnapshot
2. **PRE-HOOK** - Exec pre-backup command, wait for quiesce
3. **CHUNK & PROCESS** - Read stream, FastCDC chunk, dedup, zstd, AES-GCM, KMS wrap, RS encode
4. **STORE SHARDS** - Place shards, parallel Put, verify checksums
5. **COMMIT METADATA** - Write chunk/snapshot metadata to Raft, quorum ack
6. **SIGN MANIFEST** - Create manifest, sign with operator key, store
7. **RETENTION** - Apply policy, set immutable flag, create retention lock
8. **POST-HOOK** - Exec post-backup, cleanup CSI snapshot
9. **COMPLETE** - Update Backup CR status, emit event, release lease

### Worker Task Handler

```go
type BackupHandler struct {
    chunker     chunker.Chunker
    dedup       dedup.DedupIndex
    compressor  compression.Compressor
    encryptor   crypto.Encryptor
    erasure     erasure.Encoder
    storage     storage.ShardStore
    metadata    metadata.MetadataStore
    placer      placement.Placer
    signer      crypto.Signer
}

func (h *BackupHandler) Execute(ctx context.Context, task Task) error {
    // Implement full flow above
}
```

### Checkpoint/Resume

- Track chunk sequence position in task metadata
- On restart, resume from last committed chunk
- Idempotent chunk writes (content-addressed)

### Files to Create

- `internal/worker/backup_handler.go` - Main backup execution
- `internal/worker/backup_handler_test.go` - Unit tests
- `internal/worker/hooks.go` - Pre/post hook execution
- `internal/worker/checkpoint.go` - Checkpoint/resume logic
- `internal/worker/manifest.go` - Manifest creation/signing
- `cmd/aegis-worker/main.go` - Worker entry point
- `test/integration/backup_test.go` - End-to-end backup test

## Acceptance Criteria

- [ ] Full backup flow executes successfully
- [ ] Pre/post hooks execute in application pod
- [ ] CSI VolumeSnapshot integration works
- [ ] Chunking, dedup, compress, encrypt, encode pipeline works
- [ ] Shards stored across multiple nodes
- [ ] Metadata committed to Raft with quorum
- [ ] Manifest signed and stored
- [ ] Retention lock applied
- [ ] Checkpoint/resume works on interruption
- [ ] Backup CR status updated throughout
- [ ] Metrics: backup duration, throughput, chunk stats

## Test Cases

1. **Simple backup** - Single PVC, no hooks, completes
2. **With pre-hook** - Pre-hook executes, backup waits
3. **With post-hook** - Post-hook executes after commit
4. **Deduplication** - Second backup of same data → refs only
5. **Large volume** - 100GB, checkpoints work
6. **Interruption resume** - Kill worker mid-backup, resume
7. **Hook timeout** - Pre-hook times out → backup fails
8. **Storage failure** - One node down → shards placed elsewhere
9. **Metadata failure** - Raft unavailable → backup fails cleanly
10. **Manifest verification** - Signed manifest validates

## Dependencies

- Requires: #2, #3, #4, #5, #6, #7, #8, #9, #10, #12
- Blocks: #14, #15, #17, #18

## Definition of Done

- End-to-end backup test passes in CI
- Checkpoint/resume verified under failure injection
- Performance meets targets
- Documentation in `internal/worker/README.md`

## References

- [Implementation Plan - Task 13](../docs/implementation-plan.md#task-13---implement-backup-path)
- CSI VolumeSnapshot: https://github.com/kubernetes-csi/external-snapshotter