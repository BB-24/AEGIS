# Chunk encryption and key management

AEGIS encrypts each compressed chunk with an independently generated 32-byte
DEK using AES-256-GCM. The 12-byte random nonce and 16-byte authentication tag
are returned alongside the ciphertext. The DEK is wrapped by the configured
`KeyManager`; persist the ciphertext, nonce, wrapped DEK, and KMS key ID
together. `EncryptChunkWithContext` and `DecryptChunkWithContext` propagate
cancellation to the KMS. The context-free methods are provided for callers
that implement the simpler `Encryptor` interface.

`KMSProvider` is the integration seam for a managed KMS. `ProviderKeyManager`
adapts its encrypt/decrypt operations and delegates KEK rotation when the
provider supports it. `internal/crypto/kms.LocalProvider` is an ephemeral
development and test provider; its randomly generated KEK is process-local and
is lost on exit. It is not suitable for production or durable backups. No AWS,
Vault, Azure, or Google provider is included.

## Threat model and limitations

- GCM authenticates ciphertext and rejects modified ciphertext, nonce, or
  wrapped DEK. The Go standard library performs the cryptographic operations,
  including constant-time tag verification.
- DEKs, temporary KEK copies, and unwrapped keys are explicitly overwritten
  after use where possible. Go's garbage collector, compiler, runtime, and
  crash dumps mean this is best-effort hygiene, not a guarantee that all copies
  are removed from memory.
- Nonces and keys come from `crypto/rand` by default. A custom random source is
  exposed only for controlled environments and must be cryptographically
  secure in production.
- Chunk encryption does not provide key persistence, access control, KMS
  authorization, secure backup of KEKs, or automatic rewrapping of old DEKs.
  Production providers must preserve access to historical KEK versions or
  explicitly rewrap stored DEKs during rotation.
- This package does not claim FIPS 140-2/140-3 validation. Use a validated Go
  cryptographic module and deployment mode if compliance requires it.

Run unit tests, race tests, and fuzz seeds with:

```sh
go test ./internal/crypto/...
go test -race ./internal/crypto/...
go test -run=^$ -fuzz=FuzzAESGCMRoundTrip -fuzztime=30s ./internal/crypto
```
