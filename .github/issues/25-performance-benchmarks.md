# Issue #25: Performance Benchmarks & SLA Targets

**Epic**: Cross-Cutting | **Area**: Performance, Testing | **Priority**: P2

## Description

Define and measure performance benchmarks and SLA targets for AEGIS: backup throughput, restore RTO, metadata latency, storage efficiency, and resource utilization.

## Scope

### SLA Targets

| Metric | Target | Measurement |
|--------|--------|-------------|
| **Backup Throughput** | > 500 MB/s per worker | 100GB volume, single worker |
| **Restore Throughput** | > 500 MB/s per worker | 100GB volume, single worker |
| **RPO** | ≤ Backup interval | Configurable, default 24h |
| **RTO** | < 4 hours for 1TB | Full cluster restore |
| **Metadata Commit Latency** | < 10ms (p99) | Local cluster |
| **Metadata Read Latency** | < 5ms (p99) | ReadIndex |
| **Deduplication Ratio** | > 2:1 typical | Mixed workloads |
| **Compression Ratio** | > 1.5:1 typical | zstd level 3 |
| **Storage Overhead** | < 1.5x (RS 6,3) | 9 shards for 6 data |
| **Worker CPU** | < 2 cores per worker | At max throughput |
| **Worker Memory** | < 4 GB per worker | At max throughput |
| **Metadata Cluster** | < 2 GB per node | 1M snapshots |

### Benchmark Scenarios

1. **Single Stream Backup** - One PVC, measure throughput
2. **Concurrent Backups** - 10 PVCs, measure aggregate
3. **Incremental Backup** - Daily backup, measure dedup
4. **Full Restore** - 1TB, measure RTO
5. **Point-in-Time Restore** - Specific snapshot
6. **Metadata Stress** - 10k ops/s mixed read/write
7. **Storage Scale** - 100TB, measure capacity/efficiency
8. **Chaos Under Load** - Benchmark during failures

### Benchmark Framework

- **Tool**: Custom Go benchmarks + `hey`/`wrk` for HTTP
- **CI Integration**: Nightly benchmarks in staging
- **Regression Detection**: Alert on >10% regression
- **Reporting**: Automated benchmark reports

### Files to Create

- `benchmarks/backup_benchmark.go` - Backup throughput
- `benchmarks/restore_benchmark.go` - Restore throughput
- `benchmarks/metadata_benchmark.go` - Metadata latency
- `benchmarks/storage_benchmark.go` - Storage efficiency
- `benchmarks/runner.go` - Benchmark orchestration
- `benchmarks/reporter.go` - Report generation
- `.github/workflows/benchmarks.yaml` - Nightly benchmark CI
- `docs/performance/sla-targets.md` - SLA documentation
- `docs/performance/benchmark-results.md` - Historical results

## Acceptance Criteria

- [ ] All SLA targets defined with measurement methodology
- [ ] Benchmark suite runs automatically nightly
- [ ] Regression detection alerts on degradation
- [ ] Historical results tracked and visualized
- [ ] Resource utilization within targets
- [ ] Bottlenecks identified and documented

## Test Cases

1. **Backup throughput** - Meets 500 MB/s target
2. **Restore throughput** - Meets 500 MB/s target
3. **RTO** - 1TB restore < 4 hours
4. **Metadata latency** - p99 < 10ms commit, < 5ms read
5. **Dedup ratio** - > 2:1 on test dataset
6. **Compression ratio** - > 1.5:1 on test dataset
7. **Storage overhead** - < 1.5x with RS(6,3)
8. **Resource usage** - CPU/memory within limits
9. **Regression detection** - CI catches 10% regression
10. **Scale test** - 100TB metadata performance

## Dependencies

- Requires: #13, #14, #17
- Blocks: None

## Definition of Done

- Benchmark suite in CI
- SLA targets met in staging
- Performance regression process established
- Documentation in `benchmarks/README.md`

## References

- [Implementation Plan - Section 15](../docs/implementation-plan.md#15-performance-targets)