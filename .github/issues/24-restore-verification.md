# Issue #24: Restore Verification & Integrity Checks

**Epic**: Cross-Cutting | **Area**: Restore, Security | **Priority**: P0

## Description

Implement comprehensive restore verification: manifest signature verification, metadata commit verification, chunk hash verification, streaming integrity, and explicit failure on any mismatch.

## Scope

### Verification Layers

1. **Manifest Signature**
   - Verify operator signature on manifest
   - Check public key matches trusted key
   - Fail if signature invalid or key revoked

2. **Metadata Commit**
   - Verify snapshot metadata committed in Raft
   - Check quorum acknowledgment
   - Verify retention lock not expired/released

3. **Chunk Integrity**
   - For each chunk: fetch shards, decode, decrypt, decompress
   - Compute SHA-256 of plaintext
   - Compare with manifest chunk hash
   - Fail immediately on mismatch

4. **Streaming Verification**
   - Verify hash incrementally during stream
   - Don't buffer entire chunk in memory
   - Fail fast on first mismatch

4. **Workload Recreation**
   - Verify Kubernetes objects match manifest
   - Check PVC data integrity
   - Readiness probes pass

### Failure Modes

| Failure | Detection | Response |
|---------|-----------|----------|
| Invalid manifest sig | Signature verification | Abort, alert |
| Stale metadata | ReadIndex + quorum check | Abort, alert |
| Chunk hash mismatch | SHA-256 compare | Abort, alert |
| Shard corruption | CRC32C + RS decode fail | Repair attempt, then abort |
| KMS unwrap fail | KMS error | Retry with backoff, then abort |
| Decompress fail | zstd error | Abort, alert |

### Guarantees

- **Never return silently corrupted data**
- **Fail explicit** with actionable error messages
- **Atomic**: Partial restore leaves no orphaned resources
- **Auditable**: All verification steps logged

### Files to Create

- `internal/restore/verifier.go` - Verification orchestration
- `internal/restore/manifest_verifier.go` - Signature verification
- `internal/restore/chunk_verifier.go` - Hash verification
- `internal/restore/stream_verifier.go` - Streaming hash
- `internal/restore/workload_verifier.go` - K8s object verification
- `test/integration/restore_verification_test.go` - Verification tests

## Acceptance Criteria

- [ ] Manifest signature verified before any data fetch
- [ ] Metadata commit verified via ReadIndex
- [ ] Every chunk hash verified against manifest
- [ ] Streaming verification works for large chunks
- [ ] Any mismatch → immediate failure with clear error
- [ ] No partial data exposed on failure
- [ ] Metrics: verification duration, failures by type
- [ ] Fuzz testing finds no bypasses

## Test Cases

1. **Valid restore** - All verifications pass
2. **Tampered manifest** - Signature fails → abort
3. **Stale snapshot** - Metadata not committed → abort
4. **Corrupted chunk** - Hash mismatch → abort
5. **Missing shards** - Repair attempted → success or abort
6. **KMS failure** - Unwrap fails → retry → abort
7. **Large chunk** - Streaming verify works
8. **Concurrent restores** - Isolated verification
9. **Fuzz testing** - Malformed data handled safely
10. **Performance** - Verification overhead < 10%

## Dependencies

- Requires: #3, #6, #9, #13, #14
- Blocks: #14, #19

## Definition of Done

- All verification layers implemented and tested
- Chaos testing validates no silent corruption
- Error messages actionable for operators
- Documentation in `internal/restore/README.md`

## References

- [Implementation Plan - Section 11](../docs/implementation-plan.md#11-restore-verification)