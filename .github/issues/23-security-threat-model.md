# Issue #23: Security Engineering & Threat Model

**Epic**: Cross-Cutting | **Area**: Security, Design | **Priority**: P0

## Description

Document the comprehensive threat model and security engineering for AEGIS. Identify threats, mitigations, trust boundaries, and security requirements.

## Scope

### Threat Model (STRIDE)

| Threat | Assets at Risk | Mitigation |
|--------|----------------|------------|
| **Spoofing** | Workload identity, API auth | SPIFFE/SPIRE mTLS, SVID verification |
| **Tampering** | Backup data, metadata, manifests | AES-GCM, signed manifests, hash chains |
| **Repudiation** | Audit log, approval records | Tamper-evident audit log, signed approvals |
| **Information Disclosure** | Backup data, keys, metadata | Encryption at rest/transit, KMS envelope |
| **Denial of Service** | Metadata cluster, storage, scheduler | Quorum, rate limiting, circuit breakers |
| **Elevation of Privilege** | Admin compromise, KMS access | Dual approval, least privilege, RBAC |

### Trust Boundaries

1. **Workload Domain** - Untrusted (apps, PVCs, CSI)
2. **Control Plane** - Trusted for commands, not storage delete
3. **Metadata Domain** - Strongly consistent, quorum-backed
4. **Storage Domain** - Receives encrypted shards only
5. **Security Domain** - KMS, SPIRE, approval authority

### Security Requirements

- **Zero-trust**: No component trusted by default
- **Immutability**: Backup data cannot be altered/deleted without dual approval
- **Encryption**: AES-256-GCM per chunk, envelope with KMS
- **Integrity**: SHA-256 verification on every read
- **Audit**: Tamper-evident log of all security events
- **Key Management**: KEK rotation, DEK per chunk, HSM-backed

### Compliance Targets

- SOC 2 Type II
- GDPR (data protection, right to erasure with approval)
- HIPAA (if healthcare workloads)
- FIPS 140-2 (for crypto modules)

### Files to Create

- `docs/security/threat-model.md` - STRIDE analysis
- `docs/security/trust-boundaries.md` - Trust boundary diagram
- `docs/security/requirements.md` - Security requirements
- `docs/security/compliance.md` - Compliance mapping
- `docs/security/key-management.md` - Key hierarchy and rotation
- `internal/security/threat_model.go` - Threat model constants

## Acceptance Criteria

- [ ] STRIDE analysis complete for all components
- [ ] Trust boundaries documented with data flows
- [ ] Security requirements traceable to implementation
- [ ] Compliance gaps identified
- [ ] Key management design reviewed
- [ ] Threat model reviewed by security team

## Dependencies

- Requires: #1, #3, #5, #9, #15
- Blocks: #15, #16

## Definition of Done

- Threat model document approved
- Security requirements in implementation issues
- Compliance checklist created

## References

- [Implementation Plan - Section 14](../docs/implementation-plan.md#14-security-engineering-and-threat-model)
- STRIDE: https://docs.microsoft.com/en-us/azure/security/develop/threat-modeling-tool-threats
- NIST CSF: https://www.nist.gov/cyberframework