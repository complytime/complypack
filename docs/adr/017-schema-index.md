# ADR 017: Schema Index Replaces Embedded Schemas

**Status:** Accepted
**Supersedes:** ADR 003

**Date:** 2026-06-26

**Context:**

ADR 003 established extensible platform schemas with built-in schemas embedded in the binary and user-provided schemas via configuration. This design had three problems:

1. **Maintenance burden.** Five platforms were hardcoded with hand-maintained CUE files and pre-generated JSON. Adding a platform required writing CUE, generating JSON, modifying Go code, and cutting a release.

2. **Broken schemas.** The CI schema was a union of GitHub Actions and GitLab CI fields with a catch-all `[string]: #Job | [...string] | _` that made contract validation useless (#33, #68). The Kubernetes schema covered only Deployment, Pod, and Service.

3. **Single-schema validation.** `complypack pack` validated all policies against only `Schemas[0]`, breaking multi-platform packs (#67).

The CUE Central Registry now publishes curated schemas for GitHub Actions, GitLab CI, Azure Pipelines, and Kubernetes (as a monorepo with API groups as subpackages). These are maintained by the CUE project, not us.

**Decision:**

Replace all embedded schemas with a **schema index** — an embedded `schemas/index.yaml` that maps platform names to CUE Central Registry module sources.

```yaml
ci-github-actions:
  source: "cue://cue.dev/x/githubactions@v0#Workflow"
kubernetes-deployment:
  source: "cue://cue.dev/x/k8s.io@v0/api/apps/v1#Deployment"
```

Key design choices:

- **No embedded CUE or JSON schema files.** All schemas resolve from the CUE registry or user-provided sources at runtime.
- **Platform names are specific.** `ci` splits into `ci-github-actions`, `ci-gitlab`, `ci-azure-pipelines`. `kubernetes` splits into per-resource-type entries (`kubernetes-deployment`, `kubernetes-pod`, etc.).
- **Platforms without upstream modules are dropped.** Terraform, Docker, and Ansible have no CUE registry modules. Users register them via `--schema platform=file://path.cue`.
- **Index defaults are overridable.** `--schema kubernetes-deployment=cue://custom/schema#Deployment` overrides the index entry.
- **Multi-schema validation.** `prepack.Validate` accepts `[]cue.Value`. A policy passes if it produces zero contract violations against at least one schema.

ADR 003's core principle — extensibility via configuration — is preserved. The mechanism changes from "embedded + override" to "index defaults + override."

**Consequences:**

**Benefits:**

- Adding a platform is a one-line index entry, not a code change
- Upstream schemas are maintained by the CUE project, stay current with platform specs
- CI contract validation now works (strict upstream schemas replace the broken catch-all)
- Kubernetes schemas cover all resource types via per-API-group entries
- Multi-platform packs validate correctly

**Drawbacks:**

- Network dependency on first schema load (cached after)
- Users of bare `--schema kubernetes` or `--schema ci` must migrate to specific platform names
- Terraform, Docker, and Ansible users must provide their own schemas

**Related:**

- ADR 003: Extensible Platform Schemas (superseded)
- ADR 009: Definition Fragment Syntax
- ADR 016: Allows-Based Schema Traversal
