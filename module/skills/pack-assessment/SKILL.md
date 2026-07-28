---
name: pack-assessment
user-invocable: false
description: Use when user mentions Rego, Conftest, OPA, AMPEL, "generate policy", or "assessment logic" in the context of Gemara catalogs. Generates policies from Gemara Control Catalogs for Kubernetes (Deployments, Pods, DaemonSets, StatefulSets, CronJobs, Jobs, Services, NetworkPolicies, Ingress, RBAC, ConfigMaps, Secrets), CI/CD pipelines (GitHub Actions, GitLab CI, Azure Pipelines), Conftest, OPA, or AMPEL
---

# /comply:pack-assessment — Rego Policy Generation and Assessment

> **Internal skill** — invoked by `comply:build-assessment`, not directly by users. Use `/comply:build-assessment` or `/comply:build-assessment batch` instead.

Generate Rego policies from Gemara Control Catalogs that enforce compliance requirements.

**Core principle:** Read control definitions from source → Generate policy to satisfy approved tests → Verify it works.

## Quick Reference

| Step | Action | Output |
| ---- | ------ | ------ |
| 1. Scope | Call `get_automation_triage`, filter to automated plans | Requirement list |
| 2. Read control | Get definition from catalog (MCP) | Control text, ID, title |
| 3. Get parameters | Extract assessment requirements | Thresholds, values |
| 4. Read schema | Get platform schema (MCP) | Platform schema |
| 5. Choose format | Select evaluator output convention | Policy structure |
| 6. Generate policy | Write Rego against schema contract | .rego file |
| 7. Write to disk | Save to `policy/` | File on disk |
| 8. Generate provider mapping | Create provider-specific mapping file | `complytime-mapping.json` (OPA) or `complytime-ampel-policy.json` (Ampel) |
| 9. Validate | Contract check then test | Pass/fail results |

## Step 1: Scope — Filter to Automated Requirements

Call `get_automation_triage` with the policy name to get the automation split:

1. The tool returns `automated` and `manual` lists with requirement IDs, evaluation methods, and executor details
2. Generate policies only for requirements in the `automated` list
3. List `manual` requirements for the user — these need human review

If no policy is loaded in the MCP server, proceed with all requested requirements.

## Steps 2-5: Read and Prepare

**DO NOT generate from general knowledge.** Always read the actual control text from MCP.

1. **Read control** — get the definition from the catalog via MCP (`complypack://catalog/*`)
2. **Get parameters** — call `get_assessment_requirements` to extract thresholds and accepted values
3. **Read schema** — get the platform schema via MCP (`complypack://schema/*`)
4. **Choose format** — select evaluator output convention (e.g., OPA `deny` rules for Conftest)

## Step 6: Generate Policy — Reusability Rules

Write policies against the platform schema contract, not sample inputs:

- **Unique package namespace per file.** Each `.rego` file MUST declare a unique package namespace derived from the requirement (e.g., `package kubernetes.run_as_nonroot`). Do NOT use `package main`. The OPA provider passes this namespace as `conftest --namespace` for per-requirement evaluation — a shared namespace makes individual requirement evaluation impossible. Test files (`_test.rego`) must use the same package as their corresponding policy file. The namespace must be a valid Rego identifier — each dot-separated segment must start with a letter and contain only letters, digits, and underscores (e.g., replace hyphens with underscores).
- **Write `input.*` paths from the schema.** Read `complypack://schema/*` and use the paths it defines. Do NOT reverse-engineer paths from sample manifests in `targets/`.
- **No hardcoded values from test data.** Do not embed names, image refs, step names, or other values from sample inputs. Use parameter values from `get_assessment_requirements` for thresholds and accepted values.
- **One file per assessment requirement.** Name the file after the requirement (e.g., `run_as_nonroot.rego`).
- **Identify the subject in denial messages.** Use fields from the schema that identify the resource being checked (e.g., `input.metadata.name` for Kubernetes, `input.name` for CI jobs). Do not hardcode expected values in messages.

## Step 7: Write to Disk

Save to `policy/` directory.

## Step 8: Generate Provider-Specific Mapping File

After writing policy files to disk, generate the mapping file that links assessment plan requirement IDs to their generated policy files. The file format depends on the configured `evaluator-id` in `complypack.yaml`. This file is bundled with the policy files when published as a ComplyPack OCI artifact.

### OPA provider (`evaluator-id: opa`) — `complytime-mapping.json`

Maps Gemara Policy assessment plan requirement IDs to the Rego package namespaces of the generated checks. The opa-provider uses this file at scan time to match incoming assessment configurations to the correct Rego policies.

**Build the mapping from existing rego files.** Before generating, scan the policy output directory (e.g., `policy/`) for `.rego` files and read the `package` declaration from each one. Use these actual package namespaces as the `id` values — do not invent or assume namespaces.

1. List all `.rego` files in the policy directory
2. For each file, extract the `package` declaration (e.g., `package kubernetes.tls_version`)
3. Match each package namespace to its corresponding `requirement-id` from the Policy's `adherence.assessment-plans`
4. Write the mapping file

```json
{
  "version": "1",
  "mappings": [
    {
      "id": "kubernetes.tls_version",
      "requirement_id": "CTL-TLS-001-AR1"
    },
    {
      "id": "kubernetes.tls_ciphers",
      "requirement_id": "CTL-TLS-001-AR2"
    }
  ]
}
```

- `id` — the Rego `package` namespace declared in the `.rego` file (e.g., `package kubernetes.tls_version` → `"kubernetes.tls_version"`)
- `requirement_id` — the Gemara requirement-id from `adherence.assessment-plans[].requirement-id` in the Policy
- One entry per rego file; no duplicates in either field
- Write to `complytime-mapping.json` in the policy output directory alongside the `.rego` files

### Ampel provider (`evaluator-id: ampel`) — granular policy files + `complytime-ampel-policy.json`

Generate one JSON file per assessment requirement (granular policy), then merge them into a single `complytime-ampel-policy.json` bundle.

**Granular policy file** (one per requirement, e.g., `require-pull-request.json`):

```json
{
  "id": "require-pull-request",
  "meta": {
    "description": "Validate branch protection settings require pull/merge requests",
    "controls": [
      {
        "framework": "repo-branch-protection",
        "class": "source-code",
        "id": "pull-request-enforcement"
      }
    ]
  },
  "tenets": [
    {
      "id": "01",
      "code": "<CEL expression>",
      "predicates": {
        "types": ["<attestation predicate type URI>"]
      },
      "assessment": {
        "message": "Direct pushes are disabled. Pull/Merge requests required."
      },
      "error": {
        "message": "Direct pushes are enabled so Pull/Merge requests are not required.",
        "guidance": "Create a branch ruleset and enable 'Restrict updates'."
      }
    }
  ]
}
```

**Merged bundle** (`complytime-ampel-policy.json`):

```json
{
  "id": "complytime-ampel-policy",
  "meta": {
    "frameworks": [
      { "id": "ComplyTime-AMPEL-Policy", "name": "ComplyTime AMPEL Policy" }
    ]
  },
  "policies": [ /* all granular policies merged here */ ]
}
```

- `id` in each granular policy is the policy's semantic identity, matched against the Gemara requirement-id at scan time
- `tenets[].code` contains CEL expressions evaluated against attestation predicates
- `tenets[].predicates.types` lists the attestation predicate type URIs the tenet evaluates
- Write granular files to the policy output directory, then merge into `complytime-ampel-policy.json`

## Step 9: Validate — Contract Check Then Test Suite

1. Run `validate_policy` — confirm zero contract violations against the platform schema
2. If contract violations: fix the `input.*` paths to match the schema. The schema is the source of truth, not test data.
3. Run `test_policy` with the combined policy + test content as `policyContent` — both are in the same package, so OPA compiles and runs them together
4. If tests fail: revise the **policy**, not the tests. The tests were human-approved by `comply:test-driven-assessment` and represent the requirement.
5. Do not write the final policy artifact until all tests pass

## Safety

**DO NOT generate from general knowledge.** Always read the actual control text from MCP.

## MCP Resources and Tools

- `complypack://catalog/*` — Control Catalogs, Guidance Catalogs, Policies
- `complypack://schema/*` — Platform schemas
- `complypack://evaluator` — Available evaluators
- `get_automation_triage` — Classify assessment plans as Automated or Manual with executor details
- `get_assessment_requirements` — Extract assessment requirements with parameters
- `validate_policy` — Validate policy syntax and contract compliance
- `test_policy` — Run policy tests against sample data

## Red Flags - STOP AND FIX IF THERE ARE ISSUES

- [ ] Does every `input.*` reference exist in the platform schema?
- [ ] Are there hardcoded values from sample inputs that should be parameters?
- [ ] Did you run `validate_policy` before `test_policy`?
- [ ] Is each `.rego` file scoped to a single assessment requirement?
- [ ] Did you read control text from MCP, not from general knowledge?
- [ ] Did you call `get_automation_triage` to determine which plans are automated?
- [ ] Did you generate the provider-specific mapping file (`complytime-mapping.json` for OPA, `complytime-ampel-policy.json` for Ampel)?
- [ ] Did you avoid modifying approved test cases to make the policy pass?
- [ ] Does every `.rego` file declare a unique package namespace (not `package main`)?
