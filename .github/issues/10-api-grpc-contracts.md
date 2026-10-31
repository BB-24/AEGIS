# Issue #10: gRPC Service Contracts & Go SDK

**Epic**: Control Plane | **Area**: API, gRPC | **Priority**: P0

## Description

Define all gRPC service contracts (protobuf) for inter-component communication and generate the Go SDK. This establishes the API boundaries between data plane, metadata plane, control plane, and worker components.

## Scope

### Protobuf Definitions

```
proto/
├── storage/v1/storage.proto       # Shard storage (Put/Get/Delete/List)
├── worker/v1/worker.proto         # Worker task execution
├── metadata/v1/metadata.proto     # Metadata operations
├── scheduler/v1/scheduler.proto   # Task scheduling
├── common/v1/common.proto         # Shared types
└── admin/v1/admin.proto           # Administrative operations
```

### Services

1. **Storage Service** (from #7)
2. **Worker Service** - Execute backup/restore tasks
3. **Metadata Service** - Chunk index, snapshots, leases, manifests, locks
4. **Scheduler Service** - Task submission, status, worker registration
5. **Admin Service** - Cluster management, GC trigger, repair trigger

### Common Types

```protobuf
// proto/common/v1/common.proto
message ChunkRef {
    bytes hash = 1;           // SHA-256
    int64 logical_size = 2;
    int64 compressed_size = 3;
    repeated ShardLocation shards = 4;
}

message ShardLocation {
    string shard_id = 1;
    string node_id = 2;
    string node_address = 3;
    int32 shard_index = 4;    // 0-8
}

message Snapshot {
    string snapshot_id = 1;
    string backup_id = 2;
    string policy_id = 3;
    string namespace = 4;
    string source_volume = 5;
    string pvc_name = 6;
    int64 created_at = 7;
    int64 completed_at = 8;
    int64 total_bytes = 9;
    int64 compressed_bytes = 10;
    int64 deduplicated_bytes = 11;
    int32 chunk_count = 12;
    string manifest_digest = 13;
    SnapshotStatus status = 14;
    int64 retention_expires_at = 15;
    bool immutable = 16;
}

enum SnapshotStatus {
    SNAPSHOT_STATUS_UNSPECIFIED = 0;
    SNAPSHOT_STATUS_PENDING = 1;
    SNAPSHOT_STATUS_COMMITTED = 2;
    SNAPSHOT_STATUS_COMPLETED = 3;
    SNAPSHOT_STATUS_EXPIRED = 4;
    SNAPSHOT_STATUS_DELETED = 5;
}
```

### Go SDK Generation

- **Tool**: `buf` + `protoc-gen-go` + `protoc-gen-go-grpc`
- **Output**: `api/` directory with generated packages
- **Versioning**: Protobuf packages versioned independently
- **Compatibility**: Rules for backward/forward compatibility

### Files to Create

- `proto/storage/v1/storage.proto`
- `proto/worker/v1/worker.proto`
- `proto/metadata/v1/metadata.proto`
- `proto/scheduler/v1/scheduler.proto`
- `proto/common/v1/common.proto`
- `proto/admin/v1/admin.proto`
- `buf.yaml` - Buf configuration
- `buf.gen.yaml` - Generation configuration
- `Makefile` targets: `generate`, `generate-check`
- `api/go/` - Generated Go packages (gitignored, generated in CI)

## Acceptance Criteria

- [ ] All protobuf files compile without errors
- [ ] `buf lint` passes
- [ ] `buf breaking` detects breaking changes
- [ ] Go SDK generates without errors
- [ ] Generated code compiles in dependent packages
- [ ] Versioning strategy documented
- [ ] Compatibility rules enforced in CI
- [ ] Documentation for each service

## Test Cases

1. **Proto compilation** - All protos compile
2. **Lint** - buf lint passes
3. **Breaking change detection** - buf breaking catches incompatible changes
4. **Go generation** - `make generate` produces valid Go code
5. **Cross-language** - Generate Python/TypeScript SDKs (optional)
6. **Service integration** - Storage worker calls metadata service

## Dependencies

- Requires: #1 (Scaffolding)
- Blocks: #7 (Storage Node), #9 (Metadata), #11 (Operator), #12 (Scheduler)

## Definition of Done

- All protobuf definitions complete and reviewed
- Buf configuration works in CI
- Go SDK generates and compiles
- API documentation generated
- Compatibility policy documented

## References

- [Implementation Plan - Task 10](../docs/implementation-plan.md#task-10---define-service-contracts)
- buf: https://buf.build/
- gRPC Go: https://grpc.io/docs/languages/go/