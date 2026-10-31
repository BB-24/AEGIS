# Issue #2: FastCDC Content-Defined Chunking

**Epic**: Foundation | **Area**: Data Plane | **Priority**: P0

## Description

Implement FastCDC (Fast Content-Defined Chunking) for variable-size chunking with content-defined boundaries. This is the foundation of the data plane - all deduplication, compression, and encryption operates on these chunks.

## Scope

### Algorithm Requirements

- **FastCDC** implementation based on the paper "FastCDC: A Fast and Efficient Content-Defined Chunking Approach for Data Deduplication"
- **Target chunk size**: 64 KB (configurable: 32KB - 128KB)
- **Minimum chunk size**: 16 KB
- **Maximum chunk size**: 256 KB
- **Gear-based hash** for fast boundary detection
- **Rolling hash** with 48-bit gear table

### Interface

```go
// internal/chunker/chunker.go
type Chunker interface {
    // Chunk reads from reader and returns chunk boundaries
    Chunk(reader io.Reader) ([]Chunk, error)
    
    // ChunkBytes chunks a byte slice (for testing)
    ChunkBytes(data []byte) []Chunk
}

type Chunk struct {
    Data     []byte
    Offset   int64
    Length   int
    Hash     [32]byte // SHA-256 of Data
    Fingerprint uint64 // Gear hash for boundary
}
```

### Configuration

```go
type Config struct {
    MinSize    int // Default: 16KB
    AvgSize    int // Default: 64KB
    MaxSize    int // Default: 256KB
    Mask       uint64 // Gear mask for boundary detection
}
```

### Implementation Details

1. **Gear table generation** - Precompute 256-entry gear table using random seed
2. **Rolling hash** - Process input byte by byte, maintain 48-bit hash
3. **Boundary detection** - When `(hash & mask) == 0`, emit chunk
4. **Size enforcement** - Respect min/max boundaries
5. **SHA-256 computation** - Compute hash for each chunk (for deduplication key)

### Files to Create

- `internal/chunker/chunker.go` - Main implementation
- `internal/chunker/chunker_test.go` - Unit tests
- `internal/chunker/gear.go` - Gear table generation
- `internal/chunker/benchmark_test.go` - Performance benchmarks

## Acceptance Criteria

- [ ] FastCDC implementation produces variable-size chunks
- [ ] Average chunk size ~64KB (within 10% tolerance)
- [ ] Min/Max size constraints enforced
- [ ] SHA-256 hash computed for each chunk
- [ ] Deterministic chunking: same input → same chunks
- [ ] Performance: > 500 MB/s on modern CPU (single-threaded)
- [ ] All unit tests pass
- [ ] Benchmarks documented

## Test Cases

1. **Basic chunking** - Verify chunk boundaries on known test data
2. **Determinism** - Same input produces identical chunks
3. **Size bounds** - No chunk smaller than MinSize or larger than MaxSize
4. **Hash correctness** - SHA-256 matches manual computation
5. **Edge cases** - Empty input, tiny input, exact boundary sizes
6. **Performance** - Benchmark throughput on 1GB random data

## Dependencies

- Requires: #1 (Project Scaffolding)
- Blocks: #4 (Deduplication), #5 (Encryption), #6 (Erasure Coding)

## Definition of Done

- Package builds and all tests pass
- Benchmarks meet performance target
- Code coverage > 80%
- Documentation in `internal/chunker/README.md`

## References

- [Implementation Plan - Task 2](../docs/implementation-plan.md#task-2---implement-fastcdc-content-defined-chunking)
- FastCDC Paper: https://www.usenix.org/conference/atc16/technical-sessions/presentation/xia
- Reference Implementation: https://github.com/xxx/fastcdc