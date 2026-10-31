# Issue #3: AES-256-GCM Cryptography Primitives

**Epic**: Foundation | **Area**: Security, Data Plane | **Priority**: P0

## Description

Implement core cryptography primitives for AEGIS: AES-256-GCM encryption with unique DEK per chunk, envelope encryption with KMS KEK, and secure key management. This is a security-critical component - all backup data passes through this.

## Scope

### Encryption Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Chunk Encryption                        │
├─────────────────────────────────────────────────────────────┤
│  Plaintext Chunk → AES-256-GCM (DEK) → Ciphertext Chunk    │
│                                    ↓                        │
│                              DEK encrypted with KEK (KMS)   │
│                                    ↓                        │
│                              Stored with chunk metadata     │
└─────────────────────────────────────────────────────────────┘
```

### Interface

```go
// internal/crypto/encrypt.go
type Encryptor interface {
    // EncryptChunk encrypts a chunk with a unique DEK
    // Returns: ciphertext, nonce, encrypted DEK (wrapped with KEK), keyID, error
    EncryptChunk(plaintext []byte) (ciphertext, nonce, wrappedDEK []byte, keyID string, err error)
    
    // DecryptChunk decrypts a chunk using wrapped DEK
    DecryptChunk(ciphertext, nonce, wrappedDEK []byte, keyID string) ([]byte, error)
    
    // GenerateDEK generates a new Data Encryption Key
    GenerateDEK() ([]byte, error)
}

type KeyManager interface {
    // WrapKey encrypts a DEK with the KEK (calls KMS)
    WrapKey(ctx context.Context, dek []byte) (wrappedDEK []byte, keyID string, err error)
    
    // UnwrapKey decrypts a wrapped DEK using KMS
    UnwrapKey(ctx context.Context, wrappedDEK []byte, keyID string) ([]byte, error)
    
    // RotateKEK initiates KEK rotation
    RotateKEK(ctx context.Context) error
}
```

### AES-256-GCM Requirements

- **Algorithm**: AES-256-GCM (hardware accelerated via Go's `crypto/aes` + `crypto/cipher`)
- **Nonce**: 12 bytes (96 bits) - randomly generated per chunk
- **Tag**: 16 bytes (128 bits) - appended to ciphertext
- **DEK**: 32 bytes (256 bits) - unique per chunk
- **Key derivation**: HKDF-SHA256 if deriving from master key

### Envelope Encryption

1. Generate random DEK (32 bytes) per chunk
2. Encrypt chunk with DEK using AES-256-GCM
3. Wrap DEK with KEK via KMS (AWS KMS, HashiCorp Vault, Azure Key Vault, GCP KMS)
4. Store: `ciphertext || nonce || wrappedDEK || keyID`
5. On decrypt: Unwrap DEK from KMS → Decrypt chunk

### KMS Abstraction

```go
type KMSProvider interface {
    Encrypt(ctx context.Context, plaintext []byte) (ciphertext []byte, keyID string, err error)
    Decrypt(ctx context.Context, ciphertext []byte, keyID string) ([]byte, error)
    GenerateDataKey(ctx context.Context, keySpec string) (plaintext, ciphertext []byte, keyID string, error)
}
```

Providers: AWS KMS, HashiCorp Vault Transit, Azure Key Vault, GCP KMS, Local (dev only)

### Files to Create

- `internal/crypto/encrypt.go` - AES-GCM encrypt/decrypt
- `internal/crypto/encrypt_test.go` - Unit tests
- `internal/crypto/keystore.go` - Key management abstraction
- `internal/crypto/keystore_test.go` - Tests
- `internal/crypto/kms/` - KMS provider implementations
  - `aws.go`, `vault.go`, `azure.go`, `gcp.go`, `local.go`
- `internal/crypto/kms_test.go` - Integration tests (with mock KMS)

## Acceptance Criteria

- [ ] AES-256-GCM encrypt/decrypt roundtrip works
- [ ] Each chunk gets unique DEK (verify no key reuse)
- [ ] Tampered ciphertext fails authentication (tag verification)
- [ ] Nonce never reused (crypto/rand for generation)
- [ ] Envelope encryption: DEK wrapped/unwrapped via KMS
- [ ] KMS provider abstraction works with mock and real providers
- [ ] KEK rotation procedure implemented
- [ ] All negative tests pass (wrong key, wrong nonce, corrupted data)
- [ ] Constant-time operations where applicable
- [ ] No key material in logs or memory dumps

## Security Requirements

- [ ] Use `crypto/rand` for all randomness
- [ ] Zeroize key material after use (use `runtime.KeepAlive` or explicit zeroing)
- [ ] Constant-time comparison for authentication tags
- [ ] No timing side-channels in DEK unwrapping
- [ ] FIPS 140-2 compatible mode (document if not)

## Test Cases

1. **Roundtrip** - Encrypt → Decrypt returns original
2. **Unique DEKs** - 1000 encryptions produce 1000 unique DEKs
3. **Tamper detection** - Flip bit in ciphertext → decrypt fails
4. **Wrong key** - Decrypt with different DEK fails
5. **Nonce reuse** - Verify nonce never reused (statistical test)
6. **KMS integration** - Mock KMS wrap/unwrap works
7. **KEK rotation** - Old wrapped DEKs still decrypt after rotation
8. **Concurrent access** - Thread-safe encrypt/decrypt
9. **Large chunks** - Test with 256KB chunks
10. **Fuzz testing** - go-fuzz for encrypt/decrypt

## Dependencies

- Requires: #1 (Project Scaffolding)
- Blocks: #5 (Envelope Encryption), #13 (Backup Workflow), #14 (Restore Workflow)

## Definition of Done

- All tests pass including fuzz tests
- Code coverage > 90%
- Security review completed
- Documentation in `internal/crypto/README.md`
- Threat model documented

## References

- [Implementation Plan - Task 3](../docs/implementation-plan.md#task-3---implement-aes-256-gcm-cryptography-primitives)
- NIST SP 800-38D (GCM)
- Go crypto/aes, crypto/cipher packages
- KMS provider APIs: AWS, Vault, Azure, GCP