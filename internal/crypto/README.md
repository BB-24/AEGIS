# Chunk encryption and key management

AEGIS encrypts each compressed chunk with an independently generated 32-byte
DEK using AES-256-GCM. The 12-byte random nonce and 16-byte authentication tag
are returned alongside the ciphertext. The DEK is wrapped by the configured
`KeyManager`; persist the ciphertext, nonce, wrapped DEK, and KMS key ID
together. `EncryptChunkWithContext` and `DecryptChunkWithContext` propagate
cancellation to the KMS. The context-free methods are provided for callers
that implement the simpler `Encryptor` interface.

`internal/crypto/kms.Provider` is the integration seam for KMS providers.
`ProviderKeyManager` adapts its wrap/unwrap operations to chunk encryption.
`kms.NewProvider` selects the AWS KMS, Vault Transit, Azure Key Vault, Google
Cloud KMS, or local development provider from `kms.Config`. Cloud providers
use their standard credential chains; use workload identity or managed identity
in production rather than embedding credentials. Grant only the permissions
needed to wrap and unwrap DEKs and administer key rotation.

The local provider supports an optional file-backed KEK for development. Keep
that file outside source control, restrict access to the service identity, and
back it up securely. The default local provider is ephemeral and unsuitable
for durable backups.

`kms.NewInstrumentedProvider` can add Prometheus operation latency and error
metrics, and `kms.NewResilientProvider` provides configurable retries and a
circuit breaker. These wrappers are opt-in; select retryable errors based on
the provider/API being wrapped to avoid retrying permanent failures.
`kms.RotateAndRewrap` rotates a provider key and rewraps records supplied by a
`RotationStore`. This is a storage integration seam: the caller must implement
enumeration and atomic persistence of the updated wrapped DEKs. Provider key
rotation does not by itself rewrap existing data keys. Retain old key versions
until all dependent DEKs have been migrated or are no longer needed.

Provider specifics:

- AWS KMS uses the configured key ID and AWS SDK credential chain.
- Vault uses the Transit API; configure its address, token/standard client
  authentication, mount path, and key name.
- Azure Key Vault uses `DefaultAzureCredential` and requires an RSA key
  compatible with RSA-OAEP-256.
- Google Cloud KMS uses Application Default Credentials and a CryptoKey
  resource name.
- Local file-backed keys are not a substitute for a managed production KMS.

The package does not claim FIPS 140-2/140-3 validation. Use a validated Go
cryptographic module and deployment mode if compliance requires this.

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
