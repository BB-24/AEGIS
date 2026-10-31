# Issue #16: Secure CI/CD Pipeline & Supply Chain

**Epic**: Platform | **Area**: CI/CD, Supply Chain | **Priority**: P1

## Description

Implement a secure delivery pipeline with build gates, comprehensive testing, SBOM generation, vulnerability scanning, container signing, transparency logging, provenance, and admission policies.

## Scope

### CI Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│                        CI PIPELINE                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. CODE CHECKOUT                                               │
│  2. LINT & FORMAT                                               │
│     ├── golangci-lint                                           │
│     ├── gofmt / goimports                                       │
│     ├── buf lint (protobuf)                                     │
│     └── markdownlint                                            │
│  3. UNIT TESTS                                                  │
│     ├── go test -race -coverprofile=coverage.out               │
│     ├── go test -fuzz (time-limited)                           │
│     └── Coverage threshold: >80%                               │
│  4. INTEGRATION TESTS                                           │
│     ├── envtest for controller tests                           │
│     ├── testcontainers for multi-service tests                 │
│     └── Run in isolated namespaces                             │
│  5. SECURITY SCANS                                              │
│     ├── govulncheck (Go vulnerabilities)                       │
│     ├── Trivy (container vulnerabilities)                      │
│     ├── gosec (static analysis)                                │
│     └── syft (SBOM generation)                                 │
│  6. BUILD                                                       │
│     ├── Multi-arch: linux/amd64, linux/arm64                   │
│     ├── Reproducible builds                                    │
│     └── GoReleaser for releases                                │
│  7. SIGN & ATTEST                                               │
│     ├── Cosign sign container images                           │
│     ├── Cosign sign artifacts (SBOM, provenance)               │
│     ├── Rekor transparency log                                 │
│     └── SLSA provenance generation                             │
│  8. PUBLISH                                                     │
│     ├── Push to registry (ghcr.io/aegis/*)                     │
│     ├── Push SBOM to registry                                  │
│     └── Push provenance to registry                            │
│  9. ADMISSION CHECK                                             │
│     ├── Kyverno/OPA verify signatures                          │
│     ├── Verify SBOM matches image                              │
│     └── Only signed artifacts deploy                           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Supply Chain Artifacts

| Artifact | Tool | Format |
|----------|------|--------|
| SBOM | syft | CycloneDX JSON |
| Vulnerability Scan | Trivy | SARIF |
| Container Signature | Cosign | dsse envelope |
| Transparency Log | Rekor | Signed entry |
| Provenance | SLSA | in-toto attestation |
| Policy | Kyverno | ClusterPolicy |

### Files to Create

- `.github/workflows/ci.yaml` - Main CI pipeline
- `.github/workflows/release.yaml` - Release pipeline
- `.github/workflows/dependency-update.yaml` - Dependabot
- `cosign.pub` - Public key for verification
- `policy/kyverno/*.yaml` - Admission policies
- `scripts/sbom.sh` - SBOM generation script
- `scripts/provenance.sh` - Provenance generation
- `scripts/verify.sh` - Verification script
- `.goreleaser.yaml` - Release configuration
- `Dockerfile.*` - Multi-stage build files

## Acceptance Criteria

- [ ] CI runs on every PR and push
- [ ] All lint, test, security gates pass
- [ ] SBOM generated for every build (CycloneDX)
- [ ] Trivy scan blocks on CRITICAL/HIGH vulns
- [ ] Container images signed with Cosign
- [ ] Signatures verified in Rekor transparency log
- [ ] SLSA provenance generated (level 3 target)
- [ ] Kyverno/OPA policies admit only signed images
- [ ] Reproducible builds verified
- [ ] Multi-arch images published
- [ ] Release automation via GoReleaser

## Test Cases

1. **PR pipeline** - All checks pass on valid PR
2. **Failing lint** - PR blocked on lint failure
3. **Failing tests** - PR blocked on test failure
4. **Vulnerability blocking** - CRITICAL vuln blocks merge
5. **Signature verification** - Unsigned image rejected
6. **SBOM verification** - SBOM matches image contents
7. **Provenance verification** - Build provenance valid
8. **Multi-arch** - Images run on amd64 and arm64
9. **Release automation** - Tag push → release created
10. **Rollback** - Failed release can be rolled back

## Dependencies

- Requires: #1 (Scaffolding)
- Blocks: #17, #18, #19

## Definition of Done

- CI/CD pipeline fully automated
- Supply chain artifacts verified in staging
- Admission policies enforced in test cluster
- Documentation in `.github/workflows/README.md`

## References

- [Implementation Plan - Task 16](../docs/implementation-plan.md#task-16---implement-secure-delivery-pipeline)
- SLSA: https://slsa.dev/
- Cosign: https://github.com/sigstore/cosign
- Rekor: https://github.com/sigstore/rekor
- Kyverno: https://kyverno.io/