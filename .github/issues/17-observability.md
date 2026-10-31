# Issue #17: Observability - Metrics, Tracing, Logging, Dashboards

**Epic**: Platform | **Area**: Observability | **Priority**: P1

## Description

Add comprehensive observability: Prometheus metrics, OpenTelemetry tracing, structured JSON logging with trace IDs, Grafana dashboards, and alert rules for backup failure, GC issues, Raft instability, and restore verification errors.

## Scope

### Metrics (Prometheus)

**Key Metrics by Component:**

| Component | Metrics |
|-----------|---------|
| **Operator** | `aegis_backup_total`, `aegis_backup_duration_seconds`, `aegis_restore_total`, `aegis_restore_duration_seconds`, `aegis_reconcile_duration_seconds` |
| **Scheduler** | `aegis_tasks_queued`, `aegis_tasks_dispatched`, `aegis_worker_utilization`, `aegis_lease_duration_seconds` |
| **Worker** | `aegis_chunk_processed_total`, `aegis_chunk_bytes_total`, `aegis_encryption_duration_seconds`, `aegis_storage_put_duration_seconds` |
| **Metadata** | `aegis_raft_commit_duration_seconds`, `aegis_raft_leader_changes`, `aegis_metadata_ops_total`, `aegis_snapshot_size_bytes` |
| **Storage** | `aegis_shards_stored_total`, `aegis_shard_bytes_total`, `aegis_storage_capacity_bytes`, `aegis_storage_used_bytes` |
| **Security** | `aegis_kms_operations_total`, `aegis_approval_requests_total`, `aegis_audit_events_total` |

### Tracing (OpenTelemetry)

- **Trace context propagation** across all gRPC calls
- **Span attributes**: task_id, snapshot_id, chunk_hash, worker_id
- **Sampling**: Tail-based for errors, 10% for success
- **Exporters**: OTLP to Tempo/Jaeger, stdout for dev

### Logging

- **Format**: JSON with `level`, `timestamp`, `trace_id`, `span_id`, `component`, `message`
- **Levels**: DEBUG, INFO, WARN, ERROR
- **Structured fields**: All key identifiers for correlation

```json
{
  "level": "info",
  "timestamp": "2024-01-15T10:00:00.123Z",
  "trace_id": "abc123",
  "span_id": "def456",
  "component": "worker",
  "message": "Chunk processed",
  "chunk_hash": "sha256:...",
  "snapshot_id": "snap-abc123",
  "duration_ms": 45
}
```

### Grafana Dashboards

1. **Cluster Overview** - Health, capacity, throughput
2. **Backup/Restore** - RPO/RTO, success rates, duration trends
3. **Metadata Cluster** - Raft leadership, replication lag, commit latency
4. **Storage Plane** - Shard distribution, capacity, repair status
5. **Security** - KMS latency, approval queue, audit events
6. **Worker Pool** - Utilization, task queue, error rates

### Alert Rules

| Alert | Condition | Severity |
|-------|-----------|----------|
| BackupFailed | `increase(aegis_backup_total{status="failed"}[5m]) > 0` | critical |
| RestoreFailed | `increase(aegis_restore_total{status="failed"}[5m]) > 0` | critical |
| RaftNoLeader | `aegis_raft_leader == 0` | critical |
| RaftReplicationLag | `aegis_raft_replication_lag > 100` | warning |
| StorageCapacity | `aegis_storage_used_bytes / aegis_storage_capacity_bytes > 0.8` | warning |
| GCStalled | `aegis_gc_last_run > 1h` | warning |
| KMSErrors | `rate(aegis_kms_operations_total{status="error"}[5m]) > 0.1` | warning |

### Files to Create

- `internal/observability/metrics.go` - Metrics definitions
- `internal/observability/tracing.go` - OTel setup
- `internal/observability/logging.go` - Structured logging
- `internal/observability/exporters.go` - Prometheus, OTLP exporters
- `deploy/grafana/dashboards/*.json` - Dashboard definitions
- `deploy/prometheus/alerts.yaml` - Alert rules
- `deploy/otel/collector.yaml` - OTel collector config

## Acceptance Criteria

- [ ] All components export Prometheus metrics
- [ ] Metrics cover full backup/restore path
- [ ] OpenTelemetry traces propagate across services
- [ ] JSON logs include trace/span IDs
- [ ] Grafana dashboards render correctly
- [ ] Alert rules fire on simulated failures
- [ ] Metrics cardinality controlled (no unbounded labels)
- [ ] Performance overhead < 5%

## Test Cases

1. **Metrics exposed** - `/metrics` endpoint returns all metrics
2. **Trace propagation** - Single trace spans all services
3. **Log correlation** - Logs searchable by trace_id
4. **Dashboard queries** - All panels show data
5. **Alert firing** - Simulated failure triggers alert
6. **Cardinality** - No label explosion under load
7. **Performance** - Overhead measured under load

## Dependencies

- Requires: #1, #9, #10, #11, #12, #13, #14
- Blocks: #18, #19

## Definition of Done

- Dashboards imported in Grafana
- Alerts configured in Alertmanager
- Distributed tracing working end-to-end
- Documentation in `internal/observability/README.md`

## References

- [Implementation Plan - Task 17](../docs/implementation-plan.md#task-17---add-observability)
- Prometheus: https://prometheus.io/
- OpenTelemetry: https://opentelemetry.io/
- Grafana: https://grafana.com/