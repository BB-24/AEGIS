# Issue #4: Chunk Deduplication & zstd Compression

**Epic**: Data Plane | **Area**: Data Plane | **Priority**: P0

## Description

Implement chunk-level deduplication using content-addressed storage (SHA-256 hash as key) and zstd compression for each chunk before encryption. This reduces storage footprint and network transfer.

## Scope

### Deduplication

- **Content-addressed**: Chunk ID = SHA-256(plaintext)
- **Reference counting**: Track how many snapshots reference each chunk
- **Index**: In-memory + persistent index (LSM tree or BoltDB) mapping hash → chunk metadata
- **Global dedup**: Across all snapshots, namespaces, clusters

### Compression

- **Algorithm**: zstd (Zstandard) - best ratio/speed tradeoff
- **Level**: 3 (default, configurable 1-19)
- **Dictionary**: Optional trained dictionary for backup workloads
- **Frame format**: zstd frame with content size for streaming

### Pipeline

```
Plaintext Chunk → SHA-256 → Check Dedup Index
    ↓ (if new)
Compress (zstd) → Encrypt (AES-GCM) → Store Shards
    ↓
Update Dedup Index (ref_count = 1)
```

### Interface

```go
// internal/dedup/dedup.go
type DedupIndex interface {
    // Lookup returns chunk metadata if exists
    Lookup(hash [32]byte) (*ChunkMetadata, bool)
    
    // Add registers a new chunk
    Add(hash [32]byte, meta ChunkMetadata) error
    
    // IncRef increments reference count
    IncRef(hash [32]byte) error
    
    // DecRef decrements reference count, returns true if zero
    DecRef(hash [32]byte) (bool, error)
    
    // Stats returns deduplication statistics
    Stats() DedupStats
}

type ChunkMetadata struct {
    Hash           [32]byte
    LogicalSize    int64
    CompressedSize int64
    EncryptedSize  int64
    RefCount       int64
    ShardLocations []ShardLocation
    State          ChunkState // Available, Repairing, Missing, Deleted
}

// internal/compression/zstd.go
type Compressor interface {
    Compress(src []byte) ([]byte, error)
    Decompress(src []byte) ([]byte, error)
    CompressToWriter(src []byte, dst io.Writer) error
    DecompressFromReader(src io.Reader, dst []byte) (int, error)
}
```

### Files to Create

- `internal/dedup/index.go` - Deduplication index (BoltDB or BadgerDB backed)
- `internal/dedup/index_test.go` - Unit tests
- `internal/compression/zstd.go` - zstd wrapper
- `internal/compression/zstd_test.go` - Tests + benchmarks
- `internal/pipeline/chunk_processor.go` - Combined dedup+compress+encrypt pipeline

## Acceptance Criteria

- [ ] Deduplication index stores and retrieves chunk metadata
- [ ] Reference counting works correctly (inc/dec)
- [ ] Concurrent access safe
- [ ] zstd compression at level 3 achieves > 2:1 ratio on typical data
- [ ] Compression/decompression roundtrip is lossless
- [ ] Pipeline: dedup → compress → encrypt works end-to-end
- [ ] Performance: > 200 MB/s compress, > 500 MB/s decompress
- [ ] Dictionary training improves ratio on backup workloads
- [ ] All tests pass including race detector

## Test Cases

1. **Dedup basic** - Same chunk twice → ref_count = 2, stored once
2. **Dedup different** - Different chunks → both stored
3. **Ref counting** - IncRef/DecRef maintains accurate count
4. **Concurrent** - Multiple goroutines adding/looking up
5. **Compression roundtrip** - Compress → Decompress = original
6. **Compression ratio** - Verify on sample datasets (text, binary, mixed)
7. **Pipeline integration** - Dedup + Compress + Encrypt end-to-end
8. **Performance benchmarks** - Throughput at various chunk sizes

## Dependencies

- Requires: #1 (Scaffolding), #2 (FastCDC), #3 (Crypto)
- Blocks: #5 (Envelope Encryption), #13 (Backup Workflow)

## Definition of Done

- All tests pass with race detector
- Benchmarks meet throughput targets
- Integration with chunker and crypto verified
- Documentation in `internal/dedup/README.md` and `internal/compression/README.md`

## References

- [Implementation Plan - Tasks 4, 5](../docs/implementation-plan.md#task-4---implement-chunk-deduplication-and-zstd-compression)
- zstd: https://facebook.github.io/zstd/
- Go zstd binding: github.com/klauspost/compress/zstd