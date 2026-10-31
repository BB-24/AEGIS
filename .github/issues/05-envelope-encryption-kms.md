# Issue #5: Envelope Encryption with KMS Integration

**Epic**: Data Plane | **Area**: Security, Data Plane | **Priority**: P0

## Description

Implement envelope encryption where each chunk has a unique DEK (Data Encryption Key) wrapped by a KEK (Key Encryption Key) managed by an external KMS. Support multiple KMS providers with a pluggable architecture.

## Scope

### Envelope Encryption Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    ENCRYPTION (Backup)                          │
├─────────────────────────────────────────────────────────────────┤
│  1. Generate random DEK (32 bytes)                              │
│  2. Encrypt chunk with DEK using AES-256-GCM                   │
│  3. Call KMS: WrapKey(DEK) → wrappedDEK + keyID                │
│  4. Store: ciphertext || nonce || wrappedDEK || keyID          │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    DECRYPTION (Restore)                         │
├─────────────────────────────────────────────────────────────────┤
│  1. Read: ciphertext, nonce, wrappedDEK, keyID                 │
│  2. Call KMS: UnwrapKey(wrappedDEK, keyID) → DEK               │
│  3. Decrypt chunk with DEK using AES-256-GCM                   │
│  4. Verify integrity (GCM tag)                                 │
└─────────────────────────────────────────────────────────────────┘
```

### KMS Provider Interface

```go
// internal/crypto/kms/provider.go
type KMSProvider interface {
    // WrapKey encrypts a DEK with the KEK
    // Returns wrapped DEK and key identifier
    WrapKey(ctx context.Context, dek []byte) (wrappedDEK []byte, keyID string, err error)
    
    // UnwrapKey decrypts a wrapped DEK
    UnwrapKey(ctx context.Context, wrappedDEK []byte, keyID string) ([]byte, error)
    
    // GetKeyID returns the current KEK identifier
    GetKeyID() string
    
    // RotateKey initiates KEK rotation (async)
    RotateKey(ctx context.Context) error
    
    // HealthCheck verifies KMS connectivity
    HealthCheck(ctx context.Context) error
}
```

### Supported Providers

| Provider | Implementation | Key Features |
|----------|---------------|--------------|
| **AWS KMS** | `kms/aws.go` | IAM auth, key policies, automatic rotation |
| **HashiCorp Vault** | `kms/vault.go` | Transit engine, token auth, lease renewal |
| **Azure Key Vault** | `kms/azure.go` | Managed HSM, RBAC, key versioning |
| **GCP KMS** | `kms/gcp.go` | IAM, CMEK, automatic rotation |
| **Local/Dev** | `kms/local.go` | File-based KEK, for development only |

### KEK Rotation

- **Automatic**: Provider-managed rotation (AWS, GCP)
- **Manual**: Operator-triggered rotation
- **Process**:
  1. Generate new KEK version in KMS
  2. Re-wrap all active DEKs (background job)
  3. Update keyID in metadata
  4. Mark old KEK version as deprecated
  5. Schedule old KEK deletion after grace period

### Files to Create

- `internal/crypto/kms/provider.go` - Interface definition
- `internal/crypto/kms/factory.go` - Provider factory/registration
- `internal/crypto/kms/aws.go` - AWS KMS implementation
- `internal/crypto/kms/vault.go` - Vault Transit implementation
- `internal/crypto/kms/azure.go` - Azure Key Vault implementation
- `internal/crypto/kms/gcp.go` - GCP KMS implementation
- `internal/crypto/kms/local.go` - Local file-based (dev)
- `internal/crypto/kms/rotation.go` - KEK rotation controller
- `internal/crypto/kms/*_test.go` - Unit tests with mocks
- `internal/crypto/envelope.go` - High-level envelope encrypt/decrypt
- `internal/crypto/envelope_test.go` - Integration tests

## Acceptance Criteria

- [ ] All 5 KMS providers implement the interface
- [ ] Factory pattern for provider selection via config
- [ ] Envelope encrypt/decrypt works end-to-end
- [ ] KEK rotation: manual and automatic modes
- [ ] Key versioning: can decrypt with old key versions
- [ ] Health checks for each provider
- [ ] Proper error handling (retry, circuit breaker)
- [ ] Metrics: wrap/unwrap latency, errors, key age
- [ ] Local provider works for development without external deps

## Configuration

```yaml
kms:
  provider: "aws"  # aws, vault, azure, gcp, local
  aws:
    region: "us-east-1"
    key_id: "arn:aws:kms:us-east-1:123456789012:key/..."
  vault:
    address: "https://vault.example.com"
    transit_path: "transit"
    key_name: "aegis-kek"
  local:
    key_file: "/etc/aegis/kek.key"
```

## Test Cases

1. **Provider factory** - Creates correct provider from config
2. **AWS KMS** - Wrap/unwrap with real/mock AWS KMS
3. **Vault Transit** - Wrap/unwrap with Vault dev server
4. **Local provider** - File-based KEK works
5. **Envelope roundtrip** - Encrypt → Decrypt with each provider
6. **KEK rotation** - Old wrapped DEKs still decrypt after rotation
7. **Concurrent access** - Multiple goroutines wrapping/unwrapping
8. **Error handling** - KMS unavailable → proper error + retry
9. **Key versioning** - Decrypt with previous key version
10. **Performance** - Wrap/unwrap latency < 50ms (local), < 200ms (remote)

## Dependencies

- Requires: #1 (Scaffolding), #3 (Crypto Primitives), #4 (Dedup/Compression)
- Blocks: #13 (Backup Workflow), #14 (Restore Workflow), #15 (Security Controls)

## Definition of Done

- All provider implementations complete with tests
- Integration tests with real KMS (in CI with test credentials)
- KEK rotation procedure documented and tested
- Documentation in `internal/crypto/kms/README.md`
- Security review of key handling

## References

- [Implementation Plan - Task 5](../docs/implementation-plan.md#task-5---implement-envelope-encryption-with-kms-integration)
- AWS KMS API, Vault Transit API, Azure Key Vault API, GCP KMS API
- NIST SP 800-57 Key Management