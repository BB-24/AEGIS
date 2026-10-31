# Issue #15: Security Controls - SPIFFE, KMS, Dual Approval, Audit

**Epic**: Security | **Area**: Security, Identity | **Priority**: P0

## Description

Implement comprehensive security controls: SPIFFE/SPIRE workload identity with mTLS, OIDC-based human approval flow, KMS access patterns with KEK rotation, dual approval for destructive retention operations, and tamper-evident audit logging.

## Scope

### 1. SPIFFE/SPIRE Workload Identity & mTLS

- **SPIRE Agent** on each node
- **SPIRE Server** for identity issuance
- **Workload API** for SVID retrieval
- **mTLS** for all gRPC communication
- **SPIFFE IDs**: `spiffe://aegis.io/ns/<ns>/sa/<service>`

```go
// internal/security/identity.go
type IdentityProvider interface {
    GetSVID(ctx context.Context) (*x509svid.SVID, error)
    GetBundle(ctx context.Context) (*bundle.Bundle, error)
    RotateSVID(ctx context.Context) error
}
```

### 2. OIDC Human Approval Flow

- **Approval Request** CRD for destructive actions
- **OIDC Integration** - Dex, Keycloak, Azure AD, Okta
- **Approval Policy** - Required approvers, groups, timeout
- **Audit Trail** - Who approved, when, from where

```yaml
apiVersion: security.aegis.io/v1
kind: ApprovalRequest
metadata:
  name: delete-snap-abc123
spec:
  action: DeleteSnapshot
  target: snap-abc123
  reason: "Compliance cleanup"
  requiredApprovers: 2
  approverGroups: ["backup-admins", "security-team"]
  expiresAt: "2024-01-16T00:00:00Z"
status:
  approvals:
    - user: "alice@example.com"
      timestamp: "2024-01-15T10:00:00Z"
    - user: "bob@example.com"
      timestamp: "2024-01-15T10:05:00Z"
  state: Approved
```

### 3. KMS Access Patterns & KEK Rotation

- **Key hierarchy**: Master KEK → Key Encryption Key → Data Encryption Keys
- **Access patterns**: Least privilege, per-component keys
- **Rotation**: Automated + manual, with re-wrap
- **Audit**: All KMS operations logged

### 4. Dual Approval for Destructive Actions

- **Actions requiring approval**:
  - Delete snapshot (before retention expiry)
  - Release retention lock
  - Delete chunk index (GC)
  - Modify BackupPolicy retention
  - Rotate KEK (manual)
- **Implementation**: Admission webhook + ApprovalRequest CRD

### 5. Audit Logging

- **Format**: Structured JSON with trace IDs
- **Tamper-evident**: Hash-chained log entries
- **Events**: All API calls, approvals, KMS ops, config changes
- **Storage**: Immutable object storage + local buffer

### Files to Create

- `internal/security/identity.go` - SPIFFE/SPIRE integration
- `internal/security/identity_test.go` - Tests
- `internal/security/approval.go` - Approval flow
- `internal/security/approval_controller.go` - ApprovalRequest controller
- `internal/security/kms_access.go` - KMS policies
- `internal/security/audit.go` - Audit logger
- `internal/security/admission_webhook.go` - Dual approval enforcement
- `config/crd/security.aegis.io_approvalrequests.yaml`
- `cmd/aegis-security/main.go` - Security controller
- `deploy/spire/` - SPIRE deployment manifests

## Acceptance Criteria

- [ ] All gRPC connections use mTLS with SPIFFE IDs
- [ ] SPIRE agent/server deployed and issuing SVIDs
- [ ] ApprovalRequest CRD works for destructive actions
- [ ] OIDC integration with at least one provider
- [ ] Dual approval enforced by admission webhook
- [ ] KMS operations use least-privilege keys
- [ ] KEK rotation procedure works
- [ ] Audit log is tamper-evident (hash chain)
- [ ] All security events logged with trace IDs
- [ ] Compromised workload cannot delete protected objects

## Test Cases

1. **mTLS handshake** - Client/server verify SPIFFE IDs
2. **SVID rotation** - Automatic rotation works
3. **Approval flow** - Request → approve → action allowed
4. **Denied approval** - Request denied → action blocked
5. **Timeout approval** - Expires → action blocked
6. **Dual approval** - Single approval insufficient
7. **KMS least privilege** - Component only accesses its keys
8. **KEK rotation** - Old DEKs still decrypt after rotation
9. **Audit tamper detection** - Modified log detected
10. **Compromised workload test** - No access to storage/metadata

## Dependencies

- Requires: #3 (Crypto), #5 (Envelope Encryption), #9 (Metadata), #10 (gRPC), #11 (Operator)
- Blocks: #16, #17, #18, #19

## Definition of Done

- Security review completed
- Penetration test passed
- Compliance documentation (SOC2, etc.)
- Documentation in `internal/security/README.md`

## References

- [Implementation Plan - Task 15](../docs/implementation-plan.md#task-15---build-security-controls)
- SPIFFE: https://spiffe.io/
- SPIRE: https://github.com/spiffe/spire