# Issue #26: Documentation - Architecture, API, Operations

**Epic**: Cross-Cutting | **Area**: Documentation | **Priority**: P2

## Description

Create comprehensive documentation for AEGIS: architecture overview, API reference, deployment guides, operations runbooks, developer guides, and user guides.

## Scope

### Documentation Structure

```
docs/
├── architecture/
│   ├── overview.md              # System overview
│   ├── data-plane.md            # Chunking, encryption, storage
│   ├── metadata-plane.md        # Raft, state machine
│   ├── control-plane.md         # Operator, scheduler
│   ├── security.md              # Threat model, trust boundaries
│   ├── distributed-systems.md   # Consistency, partitions
│   └── data-model.md            # Entity definitions
├── api/
│   ├── grpc-reference.md        # gRPC service definitions
│   ├── rest-reference.md        # REST API (if applicable)
│   ├── error-codes.md           # Error code reference
│   └── versioning.md            # API versioning policy
├── deployment/
│   ├── quickstart.md            # 5-minute deploy
│   ├── production.md            # Production deployment
│   ├── prerequisites.md         # K8s, CSI, KMS, SPIRE
│   ├── configuration.md         # All config options
│   ├── helm-chart.md            # Helm values reference
│   └── upgrade.md               # Version upgrade procedure
├── operations/
│   ├── runbooks/
│   │   ├── backup-failure.md
│   │   ├── restore-failure.md
│   │   ├── metadata-cluster-down.md
│   │   ├── storage-node-down.md
│   │   ├── gc-stalled.md
│   │   ├── dr-drill.md
│   │   └── kek-rotation.md
│   ├── monitoring.md            # Metrics, alerts, dashboards
│   ├── troubleshooting.md       # Common issues
│   └── capacity-planning.md     # Sizing guidelines
├── development/
│   ├── contributing.md          # Contribution guide
│   ├── coding-standards.md      # Go style, patterns
│   ├── testing-guide.md         # Unit, integration, chaos
│   ├── adding-component.md      # New component guide
│   └── release-process.md       # Release checklist
├── user-guide/
│   ├── backup-policy.md         # Creating policies
│   ├── scheduling-backups.md    # Cron, manual triggers
│   ├── restore-procedure.md     # How to restore
│   ├── retention.md             # Retention policies
│   ├── immutable-backups.md     # Compliance mode
│   └── monitoring-backups.md    # Status, alerts
└── security/
    ├── threat-model.md
    ├── compliance.md
    ├── key-management.md
    └── approval-workflow.md
```

### Documentation Standards

- **Format**: Markdown with frontmatter
- **Diagrams**: Mermaid.js for architecture diagrams
- **API Docs**: Generated from protobuf (buf build)
- **Versioning**: Docs versioned with releases
- **Search**: Algolia DocSearch or similar
- **Languages**: English primary, others optional

### Automation

- **Doc generation**: `make docs` generates API reference
- **Link checking**: CI validates all links
- **Spell check**: CI runs cspell
- **Diagram rendering**: Mermaid CLI in CI

### Files to Create

- All files in structure above
- `.github/workflows/docs.yaml` - Doc CI pipeline
- `mkdocs.yml` - MkDocs configuration
- `docs/.spellcheck.yaml` - Spell check config
- `scripts/generate-api-docs.sh` - API doc generator

## Acceptance Criteria

- [ ] All documentation sections complete
- [ ] API reference generated from protobuf
- [ ] Deployment guides tested end-to-end
- [ ] Runbooks validated in staging
- [ ] Developer guide enables new contributors
- [ ] User guide covers all common tasks
- [ ] CI validates links, spelling, diagrams
- [ ] Documentation deployed to docs.aegis.io (or GitHub Pages)

## Test Cases

1. **Quickstart** - New user deploys in < 10 min
2. **API reference** - All services documented
3. **Runbook execution** - Operator follows runbook successfully
4. **Link check** - No broken links
5. **Spell check** - No spelling errors
6. **Diagram render** - All Mermaid diagrams valid
7. **Version consistency** - Docs match release version
8. **Search** - Key topics findable

## Dependencies

- Requires: #1, #10, #11, #16, #17, #18, #19
- Blocks: None

## Definition of Done

- Documentation site live
- All runbooks tested
- New contributor onboarded using docs only
- Documentation maintenance process established

## References

- [Implementation Plan - Section 16](../docs/implementation-plan.md#16-documentation)