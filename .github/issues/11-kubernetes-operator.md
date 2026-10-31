# Issue #11: Kubernetes Operator & CRDs

**Epic**: Control Plane | **Area**: Operator, Kubernetes | **Priority**: P0

## Description

Build the Kubernetes operator that manages BackupPolicy, Backup, and Restore custom resources. The operator orchestrates the entire backup/restore lifecycle.

## Scope

### CRD Definitions

#### BackupPolicy
```yaml
apiVersion: aegis.io/v1
kind: BackupPolicy
metadata:
  name: daily-backup
spec:
  schedule: "0 2 * * *"  # Cron schedule
  retention:
    daily: 7
    weekly: 4
    monthly: 12
    yearly: 3
  immutable: true
  hooks:
    preBackup: ""
    postBackup: ""
  resources:
    namespaces: ["*"]
    labelSelector: {}
  storageClass: "aegis-storage"
```

#### Backup
```yaml
apiVersion: aegis.io/v1
kind: Backup
metadata:
  name: backup-2024-01-15
spec:
  policyRef: daily-backup
  volumeRef:
    name: my-pvc
    namespace: production
  manual: false
status:
  phase: Completed
  snapshotId: snap-abc123
  startedAt: "2024-01-15T02:00:00Z"
  completedAt: "2024-01-15T02:05:00Z"
  totalBytes: 1073741824
  compressedBytes: 536870912
  chunks: 16384
```

#### Restore
```yaml
apiVersion: aegis.io/v1
kind: Restore
metadata:
  name: restore-2024-01-15
spec:
  snapshotRef: snap-abc123
  targetVolume:
    name: restored-pvc
    namespace: production
    storageClass: "aegis-storage"
status:
  phase: Completed
  startedAt: "2024-01-15T10:00:00Z"
  completedAt: "2024-01-15T10:10:00Z"
  bytesRestored: 1073741824
  rtoSeconds: 600
```

### Operator Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    AEGIS OPERATOR                           │
├─────────────────────────────────────────────────────────────┤
│  Controllers:                                               │
│  ├── BackupPolicyController  - Watches BackupPolicy        │
│  ├── BackupController        - Watches Backup              │
│  ├── RestoreController       - Watches Restore             │
│  └── GarbageCollectionController - Watches expired snaps  │
├─────────────────────────────────────────────────────────────┤
│  Shared:                                                    │
│  ├── TaskClient              - Submits tasks to scheduler  │
│  ├── MetadataClient          - Reads snapshot metadata     │
│  ├── EventRecorder           - Kubernetes events           │
│  └── StatusUpdater           - Updates CR status conditions│
└─────────────────────────────────────────────────────────────┘
```

### Controller Runtime

- **Framework**: `controller-runtime` (Kubebuilder)
- **Reconcile loops**: Idempotent, handle conflicts
- **Status conditions**: Standard Kubernetes conditions
- **Events**: Emit for all state transitions
- **Finalizers**: For cleanup on deletion
- **Webhooks**: Validation + mutation (optional)

### Files to Create

- `config/crd/bases/aegis.io_backuppolicies.yaml`
- `config/crd/bases/aegis.io_backups.yaml`
- `config/crd/bases/aegis.io_restores.yaml`
- `internal/operator/controllers/backuppolicy_controller.go`
- `internal/operator/controllers/backup_controller.go`
- `internal/operator/controllers/restore_controller.go`
- `internal/operator/controllers/gc_controller.go`
- `internal/operator/task_client.go` - gRPC client to scheduler
- `internal/operator/metadata_client.go` - gRPC client to metadata
- `cmd/aegis-operator/main.go` - Operator entry point
- `config/rbac/` - RBAC manifests
- `config/manager/` - Deployment manifest
- `internal/operator/*_test.go` - Controller tests (envtest)

## Acceptance Criteria

- [ ] CRDs install and validate correctly
- [ ] BackupPolicy controller creates scheduled Backup CRs
- [ ] Backup controller submits task to scheduler, updates status
- [ ] Restore controller submits restore task, updates status
- [ ] Status conditions reflect actual state (Pending, Running, Completed, Failed)
- [ ] Events emitted for all transitions
- [ ] Finalizers clean up resources on deletion
- [ ] Webhooks validate spec (optional but recommended)
- [ ] Leader election for HA operator deployment
- [ ] Metrics: reconcile duration, queue depth, errors

## Test Cases

1. **CRD validation** - Valid/invalid specs accepted/rejected
2. **BackupPolicy → Backup** - Cron triggers Backup creation
3. **Backup lifecycle** - Pending → Running → Completed/Failed
4. **Restore lifecycle** - Pending → Running → Completed/Failed
5. **Status updates** - Conditions match actual state
6. **Concurrent reconciles** - No duplicate side effects
7. **Deletion with finalizer** - Cleanup on delete
8. **Leader election** - Two operators, one active
9. **Scheduler integration** - Task submitted, status received
10. **Metadata integration** - Snapshot metadata readable

## Dependencies

- Requires: #1 (Scaffolding), #9 (Metadata), #10 (gRPC Contracts), #12 (Scheduler)
- Blocks: #13 (Backup Workflow), #14 (Restore Workflow)

## Definition of Done

- All controllers pass envtest integration tests
- CRDs published to `config/crd/`
- Helm chart for operator deployment
- Documentation in `internal/operator/README.md`
- Runbook for operator operations

## References

- [Implementation Plan - Task 11](../docs/implementation-plan.md#task-11---build-the-operator)
- Kubebuilder: https://kubebuilder.io/
- controller-runtime: https://pkg.go.dev/sigs.k8s.io/controller-runtime