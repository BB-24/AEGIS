# Issue #20: Data Model & API Design Specification

**Epic**: Cross-Cutting | **Area**: API, Design | **Priority**: P0

## Description

Define the complete data model and API design specification for AEGIS. This serves as the contract between all components and the foundation for implementation.

## Scope

### Core Entities

**Snapshot**
- Fields: snapshot_id, backup_id, policy_id, namespace, source_volume, pvc_name, created_at, completed_at, total_bytes, compressed_bytes, deduplicated_bytes, chunk_count, manifest_digest, status, retention_expires_at, immutable
- Relationships: BackupPolicy → Snapshots, Snapshot → ChunkMetadata, Snapshot → Manifest + RetentionLock
- Lifecycle: Pending → Committed → Completed → Expired/Deleted
- Invariants: Not restore-visible until committed+quorum; immutable requires dual approval

**Chunk**
- Fields: chunk_hash (SHA-256), logical_size, compressed_size, encryption_algorithm, key_id, nonce, etag, ref_count, state
- Relationships: Chunk → Snapshot refs, Chunk → ShardLocations
- Invariants: Hash matches plaintext before encryption; never reused for different plaintext

**Shard**
- Fields: shard_id, chunk_hash, shard_index (0-8), node_id, node_address, size, checksum, created_at
- Relationships: Shard → Chunk, Shard → StorageNode

**Manifest**
- Fields: manifest_id, snapshot_id, chunk_list[], signature, public_key_id, created_at
- Purpose: Signed record for restore verification

**RetentionLock**
- Fields: snapshot_id, expires_at, immutable, approvals[], created_at
- Purpose: Enforce retention, require approvals for deletion

### API Design Principles

- **gRPC-first**: All internal communication via gRPC
- **Protobuf**: Versioned packages (v1, v2...)
- **Idempotency**: All mutating operations idempotent via keys
- **Pagination**: List operations support cursor-based pagination
- **Errors**: Rich error codes with retry guidance
- **Observability**: All APIs emit metrics, traces, structured logs

### Files to Create

- `docs/api/data-model.md` - Complete data model documentation
- `docs/api/design-principles.md` - API design guidelines
- `proto/common/v1/types.proto` - Shared type definitions
- `proto/common/v1/errors.proto` - Error code definitions
- `api/openapi.yaml` - REST API spec (if needed for external)

## Acceptance Criteria

- [ ] Data model documented with all fields, relationships, invariants
- [ ] API design principles established and reviewed
- [ ] Protobuf types defined for all entities
- [ ] Error code taxonomy complete
- [ ] Versioning strategy documented
- [ ] Compatibility rules enforced

## Dependencies

- Requires: #1 (Scaffolding)
- Blocks: #2-#19 (all implementation tasks)

## Definition of Done

- Data model reviewed by architecture team
- API design documented and versioned
- Protobuf definitions compile
- Used as reference for all implementation

## References

- [Implementation Plan - Section 8](../docs/implementation-plan.md#8-data-model)