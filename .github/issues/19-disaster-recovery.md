# Issue #19: Cold Storage & Disaster Recovery Procedures

**Epic**: Platform | **Area**: DR, Backup | **Priority**: P1

## Description

Build cold storage export and disaster recovery procedures: export signed catalogue, restore from object lock/S3, validate catalogue integrity, rebuild metadata, and run DR drills.

## Scope

### Cold Storage Export

1. **CATALOGUE GENERATION** - Enumerate snapshots, create manifest, write to object storage
2. **OBJECT LOCK / IMMUTABILITY** - Enable S3 Object Lock, set retention, legal hold
3. **CATALOGUE SIGNING** - Metadata cluster signs catalogue with timestamp

### Disaster Recovery Flow

1. **CATALOGUE VALIDATION** - Fetch, verify signature, verify manifest chain, verify chunk refs
2. **METADATA REBUILD** - Initialize Raft, replay catalogue, rebuild index, rebuild locks
3. **STORAGE VERIFICATION** - Sample shard checksums, verify erasure coding, cross-reference
4. **RESTORE VALIDATION** - Test restore samples, verify RTO, verify integrity
5. **DR DRILL** - Monthly schedule, full restore to isolated namespace, report generation

### DR Controller

```go
type DRController interface {
    ExportCatalogue(ctx context.Context, dest string) error
    ValidateCatalogue(ctx context.Context, source string) (*CatalogueReport, error)
    RebuildMetadata(ctx context.Context, catalogue string) error
    RunDRDrill(ctx context.Context, targetNamespace string) (*DRDrillReport, error)
}
```

### Files to Create

- `internal/dr/catalogue.go` - Catalogue generation/validation
- `internal/dr/rebuild.go` - Metadata rebuild from catalogue
- `internal/dr/drill.go` - DR drill orchestration
- `internal/dr/controller.go` - DR controller
- `cmd/aegis-dr/main.go` - DR CLI tool
- `test/integration/dr_test.go` - DR integration test
- `deploy/dr/` - DR procedure documentation

## Acceptance Criteria

- [ ] Catalogue export creates signed, complete snapshot index
- [ ] Object Lock enabled on cold storage bucket
- [ ] Catalogue signature verified by metadata cluster
- [ ] Metadata rebuild from catalogue works
- [ ] DR drill completes within RTO SLA
- [ ] Restored data passes integrity verification
- [ ] DR drill runs automatically on schedule
- [ ] DR report generated with metrics

## Test Cases

1. **Catalogue export** - All snapshots included, signed
2. **Catalogue validation** - Tampered catalogue detected
3. **Metadata rebuild** - Empty cluster to full metadata
4. **Storage verification** - Shard sampling detects issues
5. **DR drill** - Full restore to new namespace
6. **RTO measurement** - Meets target (e.g., < 4 hours)
7. **RPO measurement** - Data loss < backup interval
8. **Object Lock** - Objects cannot be deleted during retention
9. **Cross-region** - Catalogue in different region
10. **Automated schedule** - Drill runs monthly

## Dependencies

- Requires: #7, #9, #13, #14, #16, #17, #18

## Definition of Done

- DR drill passes in staging environment
- RTO/RPO targets met and documented
- Runbook for DR procedures complete
- Documentation in `internal/dr/README.md`

## References

- [Implementation Plan - Task 19](../docs/implementation-plan.md#task-19---build-cold-storage-and-dr-procedure)
- S3 Object Lock: https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock.html