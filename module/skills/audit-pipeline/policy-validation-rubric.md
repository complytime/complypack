# Policy Validation Rubric

Structural checks for a compiled Gemara Policy YAML. This rubric is consumed
by a context-isolated subagent during Step 6a of the adherence workflow.

The subagent receives ONLY the contents of `.complytime/child-policy.yaml`
and this rubric. It receives no delta report, no MCP resources, and no
conversation history.

## Instructions

Read the Policy YAML. Run each check below against the document. For each
check, output one line:

- `PASS — <check name>` when the check succeeds
- `FAIL — <check name>: <failing field path>` when the check fails

After all checks, output either `ALL CHECKS PASSED` (if every check is PASS)
or list the failing checks for remediation.

## Checks

### 1. No url in mapping-references

Every entry in `metadata.mapping-references` SHALL omit the `url` field
entirely. Local `file://` paths become broken references after OCI packaging,
and complypack resolves artifacts by `id` alone.

**Failing example** (has `url` — FAIL):
```yaml
mapping-references:
  - id: catalog-nist-800-53
    title: NIST SP 800-53 Rev 5
    version: "5.1.1"
    url: "file:///home/user/catalogs/nist-800-53.yaml"  # FAIL — remove this field
```

**Passing example** (no `url` — PASS):
```yaml
mapping-references:
  - id: catalog-nist-800-53
    title: NIST SP 800-53 Rev 5
    version: "5.1.1"
```

### 2. Required mapping-reference fields

Every entry in `metadata.mapping-references` SHALL contain `id`, `title`,
and `version`.

### 3. Import-reference consistency

Every `reference-id` in `imports.catalogs` and `imports.guidance` SHALL have
a corresponding entry in `metadata.mapping-references` with a matching `id`.

### 4. Assessment plan completeness

Every entry in `adherence.assessment-plans` SHALL contain `id`,
`requirement-id`, `frequency`, and `evidence-requirements`.

### 5. Parameter structure

Every entry in `adherence.assessment-plans[].parameters` SHALL contain `id`,
`label`, `accepted-values`, and `description`.

### 6. Executor on automated evaluation methods

Every evaluation method with `mode: Automated` — whether in the global
`adherence.evaluation-methods` or in a per-plan `evaluation-methods` override —
SHALL include an `executor` block with at least `id` and `name`. A runtime
routes automated plans to the policy engine named by `executor.id`; a
missing executor causes a runtime connectivity failure.

**Failing example** (automated, no `executor` — FAIL):
```yaml
evaluation-methods:
  - id: eval-opa
    type: Behavioral
    mode: Automated
```

**Passing example** (automated, with `executor` — PASS):
```yaml
evaluation-methods:
  - id: eval-opa
    type: Behavioral
    mode: Automated
    executor:
      id: opa
      name: Open Policy Agent
      type: Software
```

Evaluation methods with `mode: Manual` SHALL NOT have an `executor` — skip
this check for manual methods.
