---
name: build-assessment
description: Use when user mentions Rego, Conftest, OPA, "generate policy", or "assessment logic" in the context of Gemara catalogs. Generates Rego policies from Gemara Control Catalogs for Kubernetes (Deployments, Pods, DaemonSets, StatefulSets, CronJobs, Jobs, Services, NetworkPolicies, Ingress, RBAC, ConfigMaps, Secrets), CI/CD pipelines (GitHub Actions, GitLab CI, Azure Pipelines), Conftest, or OPA. Accepts mode argument: "single" (default, one requirement at a time) or "batch" (all at once)
---

# /comply:build-assessment — Rego Policy Generation

Generate Rego policies from Gemara Control Catalogs. This is the single entry point for policy generation — it drives test generation, human approval, and policy creation.

**Core principle:** Test cases first, human approval, then policy. Read all data from MCP, never from model memory.

## Mode Selection

Parse the mode from the skill argument:

- `/comply:build-assessment` — single mode (default)
- `/comply:build-assessment single` — explicit single mode
- `/comply:build-assessment batch` — batch mode

## Prerequisite

MCP must be configured (`comply:setup`). If `complypack.yaml` does not exist or the complypack MCP server is unreachable, stop and invoke `comply:setup` first.

## Step 1: Scope — Get the Requirement List

1. Call `get_automation_triage` with the policy name
2. Collect the `automated` list — these are the requirements to generate policies for
3. Note the `manual` list — inform the user these need human review, not policy generation
4. If no automated requirements, inform the user and stop

## Single Mode

Process one requirement at a time. Show **"Requirement N of M"** at every pause point.

### Before the Loop: Ask About Helpers

Ask once before starting the per-requirement loop. Use `AskUserQuestion`:

- header: "Helpers"
- question: "Do you have any reusable code (Rego functions, data files, patterns) to use across these policies?"
- options: "No, generate fresh" / "Yes, I'll provide them"

If the user selects "Yes", wait for them to provide the helpers. Note them for use in all test and policy drafts. The user can also offer new helpers during any requirement's review — incorporate them going forward.

### Pause 1: Present the Requirement

Show:
- Counter: **"Requirement N of M"**
- Requirement ID
- Description (from the control definition read via MCP — `complypack://catalog/*`)

Then use `AskUserQuestion`:

- header: "Requirement"
- question: "Requirement N of M: `<ID>` — <title>. Proceed with this requirement?"
- options: "Continue" (generate checks for this requirement) / "Skip" (move to the next requirement)

Do not suggest checks, show code, or take any action until the user responds.

### Pause 2: Suggest Checks and Show Variable Usage

1. Call `get_assessment_requirements` to extract parameter values and thresholds
2. Read the platform schema via MCP (`complypack://schema/*`) for valid `input.*` paths

Show:
- Suggested deny check — what condition should trigger a violation, stated in plain language
- Suggested allow check — what condition should pass cleanly, stated in plain language
- Parameter values from `get_assessment_requirements` and how they will be used as expected values in test assertions
- The `input.*` paths from the platform schema that will be referenced

Then use `AskUserQuestion`:

- header: "Checks"
- question: "Do these checks and variables look correct?"
- options: "Looks good" (proceed to test draft) / "Adjust" (I'll describe what to change)

If the user selects "Adjust", wait for their changes, apply them, and re-present.

### Pause 3: Show Draft Test for Approval

Generate a complete `_test.rego` file with:
- A unique package namespace derived from the requirement (e.g., `package kubernetes.run_as_nonroot`) and `import rego.v1`. Do NOT use `package main` or a shared name like `package policy` — the OPA provider passes each namespace as `conftest --namespace` for per-requirement evaluation, so every policy file must have its own namespace. The test file must use the same package as its corresponding policy file. The namespace must be a valid Rego identifier — each dot-separated segment must start with a letter and contain only letters, digits, and underscores (e.g., replace hyphens with underscores).
- One `test_deny_*` function — input that should trigger a violation, using `with input as {...}` with inline test data
- One `test_allow_*` function — input that should pass cleanly, using `with input as {...}` with inline test data
- One-line comment per test function stating what it checks in plain language
- Test data JSON paths from the platform schema
- Parameter values from `get_assessment_requirements`, not sample data
- Incorporate any helpers provided before the loop

Show the complete test file to the user. Then use `AskUserQuestion`:

- header: "Test"
- question: "Approve this test case for `<requirement_id>`?"
- options: "Approve" (write to disk and proceed) / "Revise" (I'll describe what to change)

**This is the hard gate.** If the user selects "Revise", wait for their changes, revise, and re-present with `AskUserQuestion` again. Do not proceed until the user selects "Approve".

On approval, write the test file to `policy/<requirement>_test.rego`.

### Pause 4: Generate Policy and Recommend Review

1. Generate the Rego policy file:
   - Read control definition from MCP (`complypack://catalog/*`)
   - Use parameter values from `get_assessment_requirements` for thresholds and accepted values
   - Write `input.*` paths from the platform schema — do not reverse-engineer from sample manifests
   - Incorporate any helpers provided before the loop
   - One file per requirement, named `policy/<requirement>.rego`
   - Declare the same unique package namespace used in the test file (e.g., `package kubernetes.run_as_nonroot`). Do NOT use `package main`.
   - Use `deny contains msg if { ... }` format with `import rego.v1`
   - Identify the subject in denial messages using schema fields (e.g., `input.metadata.name`)
   - No hardcoded values from test data
2. Run `validate_policy` — confirm zero contract violations
3. If contract violations: fix `input.*` paths to match the schema and re-validate
4. Run `test_policy` with combined policy + test content as `policyContent`
5. If tests fail: revise the **policy**, not the tests — the tests were human-approved
6. Repeat validation until all tests pass
7. Write the passing policy to `policy/<requirement>.rego`

Show the complete `.rego` policy file and test pass confirmation. Then use `AskUserQuestion`:

- header: "Policy"
- question: "Policy for `<requirement_id>` passed all tests. N of M remaining. Approve?"
- options: "Approve" (move to next requirement) / "Want changes" (I'll describe what to change)

This is a soft gate. If the user selects "Want changes", revise the policy, re-run tests, and re-present with `AskUserQuestion` again.

Loop back to **Pause 1** for the next requirement.

### After Loop: Generate Provider Mapping File

After all requirements are processed and policies written, generate the mapping file. See **Generate Provider Mapping File** below.

## Batch Mode

Process all requirements at once in two phases.

### Phase 1: All Tests

1. Call `get_automation_triage` to get the automated requirement list (already done in Step 1)
2. Ask about helpers once using `AskUserQuestion` (same as single mode's pre-loop question)
3. For each automated requirement:
   - Read the control definition from MCP
   - Call `get_assessment_requirements` for parameters
   - Read the platform schema
   - Generate one `test_deny_*` and one `test_allow_*` function with inline `with input as` data
   - Each test file must declare a unique package namespace derived from the requirement (not `package main`)
4. Present all test cases together with requirement IDs and descriptions
5. Counter: **"Generated tests for N of M requirements"**
6. Use `AskUserQuestion`:
   - header: "Tests"
   - question: "Generated tests for N of M requirements. Approve all?"
   - options: "Approve all" (write to disk) / "Flag specific tests" (I'll list which ones to revise)
7. If user flags tests, revise and re-present until approved
8. Write all approved tests to `policy/*_test.rego`

### Phase 2: All Policies

1. For each approved requirement:
   - Generate Rego policy from MCP data, using helpers if provided
   - Run `validate_policy` (contract check)
   - Run `test_policy` (test suite)
   - If tests fail: revise the policy, re-run — never modify approved tests
2. Present all passing policies for review
3. Counter: **"Policies passing: N of M"**
4. Use `AskUserQuestion`:
   - header: "Policies"
   - question: "N of M policies passing. Approve all?"
   - options: "Approve all" (write to disk) / "Want changes" (I'll list which ones to revise)
5. If the user wants changes to specific policies, revise, re-test, re-present
6. Write all passing policies to `policy/*.rego`
7. Generate the provider mapping file — see **Generate Provider Mapping File** below

## Generate Provider Mapping File

After all policy files are written to disk (in either single or batch mode), generate the mapping file that links assessment plan requirement IDs to their generated policy files. The file format depends on the configured `evaluator-id` in `complypack.yaml`. This file is bundled with the policy files when published as a ComplyPack OCI artifact.

**Important:** Only `.rego` policy files and the mapping file are included in the published bundle. `*_test.rego` files are automatically excluded from bundles by the packer — do not include them in the mapping.

### OPA provider (`evaluator-id: opa`) — `complytime-mapping.json`

Maps Gemara Policy assessment plan requirement IDs to the Rego package namespaces of the generated checks. The opa-provider uses this file at scan time to match incoming assessment configurations to the correct Rego policies.

**Build the mapping from existing rego files.** Before generating, scan the policy output directory (e.g., `policy/`) for `.rego` files (excluding `*_test.rego`) and read the `package` declaration from each one. Use these actual package namespaces as the `id` values — do not invent or assume namespaces.

1. List all `.rego` files in the policy directory (skip `*_test.rego` files)
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

### Ampel provider (`evaluator-id: ampel`) — `complytime-ampel-policy.json`

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

## MCP Resources and Tools

- `complypack://catalog/*` — Control Catalogs, Guidance Catalogs, Policies
- `complypack://schema/*` — Platform schemas
- `complypack://evaluator` — Available evaluators
- `get_automation_triage` — Classify assessment plans as Automated or Manual
- `get_assessment_requirements` — Extract assessment requirements with parameters
- `validate_policy` — Validate policy syntax and contract compliance
- `test_policy` — Run policy tests

## Red Flags — STOP AND FIX IF THERE ARE ISSUES

- [ ] MCP is unreachable — **STOP.** Do not generate from general knowledge. Inform the user and wait.
- [ ] No approved test files before policy generation — **STOP.** Tests come first.
- [ ] `input.*` paths not in the platform schema — **STOP.** Fix to match schema.
- [ ] Hardcoded values from sample inputs instead of parameters — **STOP.** Use `get_assessment_requirements`.
- [ ] Approved test cases modified to make policy pass — **STOP.** Revert tests, fix policy.
- [ ] Missing deny or allow test for a requirement — **STOP.** Add the missing test.
- [ ] Control IDs or parameter values from model memory — **STOP.** Re-read from MCP.
- [ ] User has not approved test scenarios (single mode) — **STOP.** Wait for approval.
- [ ] `validate_policy` not run before `test_policy` — **STOP.** Contract check first.
- [ ] Multiple `.rego` files share the same package namespace (e.g., all use `package main`) — **STOP.** Each file must declare a unique namespace for per-requirement conftest evaluation.
- [ ] Missing `complytime-mapping.json` (OPA) or `complytime-ampel-policy.json` (Ampel) — **STOP.** Generate the provider mapping file before packing.
- [ ] `complytime-mapping.json` uses invented namespaces instead of actual `package` declarations from `.rego` files — **STOP.** Scan the files and extract real namespaces.
