# Issue #7: Storage Node gRPC Server & Shard Persistence

**Epic**: Data Plane | **Area**: Data Plane, gRPC | **Priority**: P0

## Description

Build the storage node gRPC server that accepts erasure-coded shards and persists them to local storage or object storage. This is the storage plane component that physically holds backup data.

## Scope

### gRPC Service Definition

```protobuf
// proto/storage/v1/storage.proto
syntax = "proto3";

package aegis.storage.v1;

service ShardStore {
    // Put stores a shard
    rpc Put(PutRequest) returns (PutResponse);
    
    // Get retrieves a shard
    rpc Get(GetRequest) returns (GetResponse);
    
    // Delete removes a shard
    rpc Delete(DeleteRequest) returns (DeleteResponse);
    
    // List lists shards with optional prefix
    rpc List(ListRequest) returns (stream ListResponse);
    
    // Health check
    rpc Health(HealthRequest) returns (HealthResponse);
}

message PutRequest {
    string shard_id = 1;
    bytes data = 2;
    bool verify_checksum = 3;
}

message PutResponse {
    bool success = 1;
    string error = 2;
    int64 stored_bytes = 3;
}

message GetRequest {
    string shard_id = 1;
    bool verify_checksum = 2;
}

message GetResponse {
    bytes data = 1;
    bool success = 2;
    string error = 3;
}

message DeleteRequest {
    string shard_id = 1;
}

message DeleteResponse {
    bool success = 1;
    string error = 2;
}

message ListRequest {
    string prefix = 1;
    int32 limit = 2;
}

message ListResponse {
    string shard_id = 1;
    int64 size = 2;
    int64 created_at = 3;
}
```

### Storage Backends

1. **Local filesystem** - Directory per node, shard files
2. **S3-compatible** - MinIO, AWS S3, GCS, Azure Blob
3. **Embedded DB** - BadgerDB/BoltDB for metadata + files for data

### Interface

```go
// internal/storage/server.go
type ShardStore interface {
    Put(ctx context.Context, shardID string, data []byte) error
    Get(ctx context.Context, shardID string) ([]byte, error)
    Delete(ctx context.Context, shardID string) error
    List(ctx context.Context, prefix string) ([]ShardInfo, error)
    Stats() StorageStats
}

type StorageStats struct {
    TotalShards   int64
    TotalBytes    int64
    AvailableBytes int64
    UsedBytes     int64
}
```

### Server Features

- **mTLS** for server-client authentication (SPIFFE)
- **Checksum verification** on write and read (CRC32C)
- **Concurrent access** - multiple clients
- **Backpressure** - rate limiting, queue depth
- **Metrics** - Prometheus: request latency, throughput, errors
- **Graceful shutdown** - drain connections

### Files to Create

- `proto/storage/v1/storage.proto` - Protobuf definition
- `internal/storage/server.go` - gRPC server implementation
- `internal/storage/store.go` - Storage backend implementations
  - `filesystem.go`, `s3.go`, `badger.go`
- `internal/storage/interceptors.go` - Auth, logging, metrics interceptors
- `cmd/aegis-storage/main.go` - Entry point
- `internal/storage/server_test.go` - Integration tests
- `deploy/docker/Dockerfile.storage` - Container image

## Acceptance Criteria

- [ ] gRPC server starts and serves on configured port
- [ ] Put/Get/Delete/List RPCs work correctly
- [ ] Checksum verification on write and read
- [ ] mTLS authentication enforced
- [ ] Multiple storage backends work (filesystem, S3)
- [ ] Concurrent requests handled correctly
- [ ] Metrics exposed: latency, throughput, errors, disk usage
- [ ] Health endpoint returns healthy/ready
- [ ] Graceful shutdown drains in-flight requests
- [ ] Docker image builds and runs

## Configuration

```yaml
storage:
  backend: "filesystem"  # filesystem, s3, badger
  filesystem:
    path: "/var/lib/aegis/storage"
  s3:
    endpoint: "https://s3.amazonaws.com"
    bucket: "aegis-shards"
    region: "us-east-1"
  grpc:
    address: "0.0.0.0:9001"
    mtls: true
    cert_file: "/etc/aegis/certs/storage.crt"
    key_file: "/etc/aegis/certs/storage.key"
    ca_file: "/etc/aegis/certs/ca.crt"
```

## Test Cases

1. **Put/Get roundtrip** - Store shard, retrieve, verify data
2. **Checksum verification** - Corrupt data on disk → read fails
3. **Delete** - Delete shard, verify not found
4. **List** - List shards with prefix filtering
5. **Concurrent access** - Multiple clients Put/Get simultaneously
6. **Large shards** - 50MB shards (max chunk 256KB / 6 * 9 ≈ 42KB, but test larger)
7. **Storage backends** - Test filesystem and S3 (MinIO)
8. **mTLS** - Client cert required, invalid cert rejected
9. **Metrics** - Prometheus metrics match expected
10. **Graceful shutdown** - In-flight requests complete
11. **Failure injection** - Disk full, network partition

## Dependencies

- Requires: #1 (Scaffolding), #6 (Erasure Coding), #10 (gRPC Contracts)
- Blocks: #8 (Shard Placement), #13 (Backup Workflow), #14 (Restore Workflow)

## Definition of Done

- All tests pass including integration tests with MinIO
- Docker image published to registry
- Metrics documented
- Runbook for storage node operations
- Documentation in `internal/storage/README.md`

## References

- [Implementation Plan - Task 8](../docs/implementation-plan.md#task-8---implement-storage-node-grpc-server-and-file-backed-shard-persistence)
- gRPC Go: https://grpc.io/docs/languages/go/
- SPIFFE/SPIRE for mTLS