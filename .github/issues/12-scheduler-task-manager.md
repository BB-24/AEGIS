# Issue #12: Scheduler & Task Manager

**Epic**: Control Plane | **Area**: Operator, Scheduling | **Priority**: P0

## Description

Build the scheduler and task manager that assigns backup/restore tasks to workers, manages leases, handles retries, and tracks task state.

## Scope

### Scheduler Responsibilities

1. **Task Queue** - Priority queue (cron > manual > retry)
2. **Worker Scoring** - Score workers by capacity, locality, health
3. **Lease Management** - Acquire, renew, revoke leases
4. **Retry Logic** - Exponential backoff, max retries
5. **Task State Tracking** - In-flight, completed, failed

### Worker Lease Protocol

```
┌─────────────────────────────────────────────────────────────┐
│                    TASK LEASE FLOW                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Scheduler                    Worker                        │
│     │                           │                           │
│     ├── TaskAssigned ─────────► │                           │
│     │                           │                           │
│     │ ◄───── LeaseAcquired ─────┤ (idempotent)             │
│     │                           │                           │
│     │     [Heartbeat every 10s] │                           │
│     │ ◄─────────────────────────┤                           │
│     │                           │                           │
│     │ ◄───── TaskCompleted ─────┤                           │
│     │                           │                           │
│     └── LeaseReleased ────────► │                           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Interface

```go
// internal/scheduler/scheduler.go
type Scheduler interface {
    // SubmitTask adds a task to the queue
    SubmitTask(ctx context.Context, task Task) error
    
    // GetTaskStatus returns current task status
    GetTaskStatus(ctx context.Context, taskID string) (*TaskStatus, error)
    
    // CancelTask attempts to cancel a pending/running task
    CancelTask(ctx context.Context, taskID string) error
    
    // RegisterWorker registers a worker node
    RegisterWorker(ctx context.Context, worker WorkerInfo) error
    
    // WorkerHeartbeat updates worker health
    WorkerHeartbeat(ctx context.Context, workerID string) error
}

type Task struct {
    ID          string
    Type        TaskType // Backup, Restore, GC, Repair
    Priority    int
    Payload     []byte  // Serialized task spec
    IdempotencyKey string
    CreatedAt   time.Time
    RetryCount  int
    MaxRetries  int
}

type WorkerInfo struct {
    ID          string
    Address     string
    Capacity    int64
    Used        int64
    Labels      map[string]string
    Version     string
    LastSeen    time.Time
}
```

### Scoring Algorithm

- **Capacity score**: Available capacity / total capacity
- **Locality score**: Same zone/rack as source volume
- **Health score**: Recent heartbeats, error rate
- **Load score**: Current task count vs max concurrent
- **Composite**: Weighted sum, highest wins

### Files to Create

- `internal/scheduler/scheduler.go` - Core scheduler logic
- `internal/scheduler/queue.go` - Priority task queue
- `internal/scheduler/scoring.go` - Worker scoring
- `internal/scheduler/lease.go` - Lease management
- `internal/scheduler/retry.go` - Retry with backoff
- `internal/scheduler/metrics.go` - Scheduler metrics
- `cmd/aegis-scheduler/main.go` - Scheduler entry point
- `internal/scheduler/*_test.go` - Unit tests
- `test/scheduler/integration_test.go` - Integration tests

## Acceptance Criteria

- [ ] Tasks queued and dispatched to workers
- [ ] Worker scoring selects optimal worker
- [ ] Lease acquisition is idempotent (no duplicates)
- [ ] Heartbeats detect worker failures
- [ ] Failed tasks requeued with exponential backoff
- [ ] Max retries enforced
- [ ] Task state visible via API
- [ ] Metrics: queue depth, dispatch latency, worker utilization
- [ ] HA: Multiple schedulers, leader election
- [ ] Graceful shutdown drains in-flight tasks

## Configuration

```yaml
scheduler:
  address: "0.0.0.0:9003"
  queue:
    max_size: 10000
  worker:
    heartbeat_interval: "10s"
    lease_ttl: "30s"
    max_concurrent_tasks: 4
  retry:
    base_backoff: "30s"
    max_backoff: "10m"
    max_retries: 3
  scoring:
    capacity_weight: 0.4
    locality_weight: 0.3
    health_weight: 0.2
    load_weight: 0.1
```

## Test Cases

1. **Task submission** - Task enters queue, gets dispatched
2. **Worker scoring** - Best worker selected for task
3. **Lease idempotency** - Duplicate acquire returns success
4. **Heartbeat failure** - Worker marked unhealthy, tasks reassigned
5. **Retry logic** - Failed task requeued with backoff
6. **Max retries** - Task marked failed after max retries
7. **Priority queue** - Cron tasks before manual before retries
8. **Concurrent schedulers** - Leader election, single active
9. **Graceful shutdown** - In-flight tasks complete or requeue
10. **Load test** - 1000 tasks, 10 workers, all complete

## Dependencies

- Requires: #1 (Scaffolding), #9 (Metadata), #10 (gRPC Contracts)
- Blocks: #13 (Backup Workflow), #14 (Restore Workflow)

## Definition of Done

- All tests pass including integration tests
- HA deployment verified (2+ schedulers)
- Metrics documented
- Documentation in `internal/scheduler/README.md`

## References

- [Implementation Plan - Task 12](../docs/implementation-plan.md#task-12---build-orchestration-logic)