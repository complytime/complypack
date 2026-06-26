---
name: pack
description: Use when user wants to generate Rego policies from Gemara catalogs, extract assessment requirements and parameters, or work with compliance validation for Kubernetes, Terraform, Docker, Ansible, or CI platforms
---

# /comply:pack — Rego Policy Generation and Assessment

Generate Rego policies from Gemara Control Catalogs that enforce compliance requirements. Policies must be written to disk, validated against the target platform schema, and tested with sample inputs.

**Core principle:** Read control definitions from source → Generate platform-specific policy → Write to disk → Verify it works.

## When to Use

- User requests "generate policy for control X"
- User specifies a Gemara catalog and target platform
- User mentions Conftest, OPA, or Rego
- Generating compliance policies from security frameworks

Do NOT use for:
- Writing arbitrary Rego policies (not from Gemara controls)
- Generating policies without a source catalog

## Quick Reference

| Step | Action | Output |
| ---- | ------ | ------ |
| 1. Scope | Read child policy, filter to automated plans | Requirement list |
| 2. Read control | Get definition from catalog (MCP) | Control text, ID, title |
| 3. Get parameters | Extract assessment requirements | Thresholds, values |
| 4. Read schema | Get platform schema (MCP) | CUE schema |
| 5. Choose format | OPA (allow) or Conftest (deny) | Policy structure |
| 6. Generate policy | Write Rego against schema contract | .rego file |
| 7. Write to disk | Save to `policy/` | File on disk |
| 8. Validate | Contract check then test | Pass/fail results |

## Step 1: Scope — Filter to Automated Requirements

If `.complytime/child-policy.yaml` exists, read it and filter assessment plans:

- **`mode: Automated`** plans: generate Rego for these
- **`mode: Manual`** plans: list these for the user and explain they need a different evidence collection method

If no child policy exists, proceed with all requested requirements.

## Steps 2-5: Read and Prepare

Read control definitions, parameters, schema, and choose format as before.

**DO NOT generate from general knowledge.** Always read the actual control text from MCP.

## Step 6: Generate Policy — Reusability Rules

Write policies against the platform schema contract, not sample inputs:

- **Write `input.*` paths from the schema.** Read `complypack://schema/*` and use the paths it defines. Do NOT reverse-engineer paths from sample manifests in `targets/`.
- **No hardcoded values from test data.** Do not embed container names, image refs, step names, or other values from sample inputs. Use parameter values from `get_assessment_requirements` for thresholds and accepted values.
- **One `.rego` file per assessment requirement.** Name the file after the requirement (e.g., `kubernetes_run_as_nonroot.rego`).
- **Use `input.kind`, `input.metadata.name` in messages.** Policy denial messages should identify what was checked, not what was expected.

## Step 7: Write to Disk

Save to `policy/` directory.

## Step 8: Validate — Contract Check First

1. Run `validate_policy` — confirm zero contract violations against the platform schema
2. If contract violations: fix the `input.*` paths to match the schema. The schema is the source of truth, not test data.
3. Run `test_policy` — confirm policy logic works with sample inputs

## Safety

**DO NOT generate from general knowledge.** Always read the actual control text from MCP.

## MCP Resources and Tools

- `complypack://catalog/*` — Control Catalogs, Guidance Catalogs, Policies
- `complypack://schema/*` — Platform schemas
- `complypack://evaluator` — Available evaluators
- `get_assessment_requirements` — Extract assessment requirements with parameters
- `validate_policy` — Validate policy syntax and contract compliance
- `test_policy` — Run policy tests against sample data

## Red Flags

- [ ] Does every `input.*` reference exist in the platform schema?
- [ ] Are there hardcoded values from sample inputs that should be parameters?
- [ ] Did you run `validate_policy` before `test_policy`?
- [ ] Is each `.rego` file scoped to a single assessment requirement?
- [ ] Did you read control text from MCP, not from general knowledge?
- [ ] Did you filter to `mode: Automated` plans from the child policy?
