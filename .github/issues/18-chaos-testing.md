# Issue #18: Chaos Testing & Fault Injection

**Epic**: Platform | **Area**: Testing, Chaos | **Priority**: P1

## Description

Build fault injection scenarios to validate system resilience: crash workers/storage nodes, partition Raft quorum, corrupt shards, inject latency/packet loss, and validate recovery and alerting.

## Scope

### Chaos Scenarios

| Scenario | Description | Expected Behavior |
|----------|-------------|-------------------|
| **Worker Crash** | Kill worker mid-backup | Task reassigned, checkpoint resume |
| **Storage Node Crash** | Kill storage node | Shards unavailable, repair triggered |
| **Raft Partition** | Network partition minority | Minority rejects writes, majority continues |
| **Shard Corruption** | Bit-flip stored shard | Checksum detects, repair from parity |
| **Shard Loss** | Delete shard files | Repair reconstructs from 6 shards |
| **Latency Injection** | Add 500ms latency to gRPC | Timeouts handled, retries work |
| **Packet Loss** | 10% packet drop | Retries, eventual success |
| **Disk Full** | Fill storage disk | Graceful degradation, alerts |
| **KMS Unavailable** | Block KMS endpoint | Operations queue, retry with backoff |
| **Operator Crash** | Kill operator pod | Leader election, reconciles resume |

### Chaos Framework

- **Tool**: `chaos-mesh` or custom `krkn` integration
- **CRD-based**: Define chaos experiments as Kubernetes resources
- **Automated**: Run in CI/CD pipeline on staging
- **Observable**: Metrics, logs, traces during chaos

```yaml
apiVersion: chaos-mesh.org/v1alpha1
kind: PodChaos
metadata:
  name: kill-backup-worker
spec:
  action: pod-kill
  mode: one
  selector:
    namespaces: ["aegis-system"]
    labelSelectors:
      app: aegis-worker
  duration: "30s"
  scheduler:
    cron: "@every 1h"
```

### Validation Checks

After each chaos experiment:
1. **Data integrity** - No silent corruption
2. **Availability** - Backup/restore still work
3. **Recovery** - System self-heals within SLA
4. **Alerting** - Appropriate alerts fired
5. **No split-brain** - Raft consistency maintained

### Files to Create

- `test/chaos/scenarios/*.yaml` - Chaos Mesh experiments
- `test/chaos/runner.go` - Chaos test runner
- `test/chaos/validation.go` - Post-chaos validation
- `test/chaos/reporter.go` - Chaos test reports
- `.github/workflows/chaos.yaml` - Chaos CI pipeline
- `deploy/chaos-mesh/` - Chaos Mesh deployment

## Acceptance Criteria

- [ ] All 10 chaos scenarios defined and runnable
- [ ] Automated chaos runs in CI on staging
- [ ] Validation checks pass after each scenario
- [ ] Chaos reports generated with pass/fail
- [ ] System meets failure tolerance requirements
- [ ] RTO/RPO measured under chaos
- [ ] No data loss in any scenario

## Test Cases

1. **Worker crash during backup** - Resume from checkpoint
2. **Storage node down** - Repair reconstructs shards
3. **Raft partition** - Majority continues, minority blocked
4. **Shard corruption** - Detected, repaired from parity
5. **Network latency** - Retries handle gracefully
6. **KMS outage** - Operations queue, recover when back
7. **Disk pressure** - GC triggered, alerts fire
8. **Operator failover** - No reconciliation gaps
9. **Concurrent chaos** - Multiple failures handled
10. **Long-running chaos** - System stable over hours

## Dependencies

- Requires: #7, #8, #9, #13, #14, #17
- Blocks: #19

## Definition of Done

- Chaos suite runs weekly in CI
- All scenarios documented with expected outcomes
- Failure tolerance requirements validated
- Documentation in `test/chaos/README.md`

## References

- [Implementation Plan - Task 18](../docs/implementation-plan.md#task-18---build-fault-injection-scenarios)
- Chaos Mesh: https://chaos-mesh.org/
- Krkn: https://github.com/krkn-chaos/krkn