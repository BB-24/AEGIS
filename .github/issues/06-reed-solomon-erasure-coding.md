# Issue #6: Reed-Solomon (6,3) Erasure Coding

**Epic**: Data Plane | **Area**: Data Plane | **Priority**: P0

## Description

Implement Reed-Solomon (6,3) erasure coding to split each encrypted chunk into 9 shards, any 6 of which can reconstruct the original. This provides fault tolerance for up to 3 simultaneous storage node failures.

## Scope

### Erasure Coding Parameters

- **k = 6** (data shards)
- **m = 3** (parity shards)
- **Total shards = 9**
- **Reconstruction threshold = 6** shards
- **Failure tolerance = 3** shards

### Interface

```go
// internal/erasure/reedsolomon.go
type Encoder interface {
    // Encode splits data into 9 shards
    // Returns slice of 9 shards (6 data + 3 parity)
    Encode(data []byte) ([][]byte, error)
    
    // Decode reconstructs data from available shards
    // shards: slice of 9 elements, nil for missing shards
    // At least 6 non-nil shards required
    Decode(shards [][]byte) ([]byte, error)
    
    // ShardCount returns total shards (9)
    ShardCount() int
    
    // DataShardCount returns data shards (6)
    DataShardCount() int
    
    // ParityShardCount returns parity shards (3)
    ParityShardCount() int
}
```

### Implementation Options

1. **Go implementation**: `github.com/klauspost/reedsolomon` (recommended)
2. **Custom GF(2^8)** implementation for learning/control
3. **ISA-L acceleration** for Intel CPUs (optional optimization)

### Shard Format

```
Shard Structure:
┌─────────────────────────────────────────────────────────────┐
│  Shard Header (32 bytes)                                    │
│  ├── Magic: "AEGI" (4 bytes)                               │
│  ├── Version: 1 (1 byte)                                    │
│  ├── Shard Index: 0-8 (1 byte)                             │
│  ├── Total Shards: 9 (1 byte)                              │
│  ├── Data Shards: 6 (1 byte)                               │
│  ├── Parity Shards: 3 (1 byte)                             │
│  ├── Chunk Hash: SHA-256 of original (32 bytes)            │
│  ├── Shard Checksum: CRC32C of payload (4 bytes)           │
│  └── Reserved: 17 bytes                                     │
├─────────────────────────────────────────────────────────────┤
│  Shard Payload (variable)                                   │
└─────────────────────────────────────────────────────────────┘
```

### Files to Create

- `internal/erasure/reedsolomon.go` - Wrapper around klauspost/reedsolomon
- `internal/erasure/reedsolomon_test.go` - Unit tests
- `internal/erasure/shard.go` - Shard encoding/decoding with headers
- `internal/erasure/shard_test.go` - Shard format tests
- `internal/erasure/benchmark_test.go` - Performance benchmarks

## Acceptance Criteria

- [ ] Encode produces exactly 9 shards (6 data + 3 parity)
- [ ] Decode reconstructs from any 6 valid shards
- [ ] Decode fails with clear error if < 6 valid shards
- [ ] Corrupted shard detection (checksum mismatch)
- [ ] Shard header includes all metadata for reconstruction
- [ ] Performance: > 500 MB/s encode, > 800 MB/s decode
- [ ] Memory efficient: streaming for large chunks
- [ ] All unit tests pass including corruption cases

## Test Cases

1. **Basic encode/decode** - 9 shards → reconstruct from all 9
2. **Threshold reconstruction** - Pick any 6 of 9 → reconstruct
3. **Missing shards** - Exactly 6 shards provided → works
4. **Insufficient shards** - 5 shards → clear error
5. **Corrupted shard** - One shard modified → detect via checksum
6. **Multiple corrupted** - 2 corrupted, 7 good → still works
7. **Wrong shard index** - Shards shuffled → reconstruct correctly
8. **Large data** - 256KB chunk → shards ~42KB each
9. **Small data** - 1KB chunk → shards padded correctly
10. **Concurrent** - Multiple encode/decode in parallel
11. **Fuzz testing** - Random corruption patterns
12. **Benchmarks** - Throughput at various sizes

## Dependencies

- Requires: #1 (Scaffolding), #3 (Crypto), #4 (Compression), #5 (Envelope Encryption)
- Blocks: #7 (Storage Node), #8 (Shard Placement), #13 (Backup Workflow)

## Definition of Done

- All tests pass including fuzz tests
- Benchmarks meet throughput targets
- Shard format documented with versioning strategy
- Integration with storage node verified
- Documentation in `internal/erasure/README.md`

## References

- [Implementation Plan - Task 6](../docs/implementation-plan.md#task-6---implement-reed-solomon-63-erasure-coding)
- klauspost/reedsolomon: https://github.com/klauspost/reedsolomon
- Reed-Solomon tutorial: https://en.wikipedia.org/wiki/Reed%E2%80%93Solomon_error_correction
- ISA-L: https://github.com/intel/isa-l