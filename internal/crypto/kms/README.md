# KMS providers

The `Provider` interface wraps and unwraps data encryption keys (DEKs), exposes
the current key identifier, supports rotation, and provides a health check.
Provider construction is configured with `NewProvider`:

```go
provider, err := kms.NewProvider(ctx, kms.Config{
    Provider: "aws",
    AWS:      kms.AWSConfig{Region: "us-east-1", KeyID: "alias/aegis"},
})
```

Provider names are `aws`, `vault`, `azure`, `gcp`, and `local`. The cloud
providers use their platform credential chains; prefer managed/workload
identity over long-lived credentials. The caller must configure least-privilege
permissions for key wrapping, unwrapping, and any requested rotation operation.

## Provider settings

- **AWS**: region and KMS key ID. The SDK credential chain is used. Rotation
  enables AWS-managed automatic rotation; it does not force an immediate new
  key version.
- **Vault**: address, optional token, Transit mount path, and Transit key name.
  Rotation invokes the Transit key rotation endpoint. Configure TLS and an
  authentication method suitable for the deployment.
- **Azure**: Key Vault URL and key name. `DefaultAzureCredential` is used. The
  key must support RSA-OAEP-256. Versioned key IDs are retained for unwrapping
  old DEKs; rotation creates a new key version.
- **Google Cloud KMS**: CryptoKey resource name. Application Default
  Credentials are used. Rotation creates a key version and sets it primary.
- **Local**: optional key file. Without a file, the provider is ephemeral and
  loses its KEK on process exit. File-backed key material is sensitive: keep
  the file out of source control, restrict filesystem permissions, and make
  protected backups. Use only for development or tests.

## Resilience, metrics, and rotation

`NewInstrumentedProvider` records provider/operation latency and errors in
Prometheus. Labels do not include key IDs or key material. `NewResilientProvider`
adds bounded retries and a consecutive-failure circuit breaker. Configure
`ShouldRetry` to match transient errors for your provider; the default retries
network and `Temporary()` errors only. Avoid stacking retries without
accounting for the SDK's own retry behavior.

`RotateAndRewrap` initiates provider rotation, then processes records from a
`RotationStore`. Applications must implement the store so it enumerates all
active wrapped DEKs and persists each replacement safely. Rotation is not
complete until all records are rewrapped and old key versions remain available
for any records not yet migrated. A failed operation returns an error; it may
have partially updated records and should be retried idempotently.

## Security notes

This package does not claim FIPS 140-2/140-3 validation. Never log DEKs,
wrapped-key payloads, access tokens, or provider credentials. Protect local key
files and KMS permissions, retain old key versions until rewrapping is
verified, and test restore access before retiring keys.

Run provider unit tests with:

```sh
go test ./internal/crypto/kms/...
```

Real cloud integration tests require credentials and KMS resources and are not
run by the unit test suite.
