---
name: comply-adherence
description: Populate a Gemara Policy defining what controls apply, with what parameter values, and how evidence will be collected
user-invocable: false
---

# Adherence — Compile Policy

Compile or alter a Gemara Policy artifact. Declares what controls apply, with exact parameter values, and defines the adherence plan: frequency, evaluation method, and evidence requirements.

Evidence collection and Evaluation Logs are produced by `complyctl` at runtime.

## Prerequisites

- `.complytime/scoping.yaml`
- `.complytime/delta-report.yaml`

Verify all parameters are resolved (no `pending_user_decision`). If unresolved, tell the user to re-run mapping.

## Process

### Step 1: Read Input Artifacts

Read both `.complytime/scoping.yaml` and `.complytime/delta-report.yaml`.

### Step 2: Build Mapping References

From the delta report's `sources`, build `mapping_references`:
- One for the parent Policy
- One for each scoped Control Catalog
- One for each Guidance Catalog for the target framework

### Step 3: Build Imports

- `imports.catalogs` — one per Control Catalog
- `imports.guidance` — one per mandated Guidance Catalog

### Step 4: Build Assessment Plans

#### 4a: Identify the trusted evaluator

Ask the user: "Who is the trusted evaluator for automated assessments?" Collect:
- `id` — evaluator plugin identifier (e.g., `opa`)
- `name` — display name (e.g., `OPA/Conftest`)
- `type` — `Software`
- `version` — evaluator version

This becomes the **default** executor for all automated plans, set as a global evaluation method on `adherence.evaluation-methods`. Individual assessment plans can override this with their own `evaluation-methods` and `executor` when a specific requirement needs a different evaluator or mode.

#### 4b: Triage requirements by automation eligibility

For each requirement from `get_assessment_requirements`, check its `applicability` array for the convention group ID indicating automation eligibility (e.g., `"automated"`). Read `metadata.applicability-groups` from the catalog to understand what groups exist.

- **Automatable** (applicability includes the automation group): assessment plan inherits the global `mode: Automated` and executor — no per-plan `evaluation-methods` needed unless overriding
- **Not automatable**: assessment plan overrides with per-plan `evaluation-methods` set to `mode: Manual`, no executor

The user may also override any individual plan's evaluation method and executor. Present the triage results and ask if any plans need a different evaluator or mode before compiling.

#### 4c: Build each plan

Group by `requirement-id`. Each plan:
- `id` — unique plan identifier
- `requirement-id` — the assessment requirement this plan addresses
- `frequency` (e.g., "30d", "90d", "365d")
- `evaluation-methods` — only when overriding the global default (e.g., manual plans or plans with a different executor)
- `evidence-requirements` — what evidence is collected
- `parameters` — frozen values from harmonization, each with `id`, `label`, `accepted-values`, `description`

### Step 5: Compile the Policy

```yaml
title: "<System Name> Policy"
metadata:
  id: <system-name>-policy
  gemara-version: v1.0.0
  type: Policy
  description: "<system description>"
  author:
    id: <author-id>
    name: <author-name>
    type: Software Assisted
  mapping-references:
    - id: <ref-id>
      title: "<reference title>"
      version: "<version>"
contacts:
  responsible:
    - name: "<contact name>"
  accountable:
    - name: "<contact name>"
scope:
  in:
    technologies:
      - <technology from system profile>
    groups:
      - <applicability group, e.g. maturity-1, maturity-2>
imports:
  catalogs:
    - reference-id: <ref-id>
  guidance:
    - reference-id: <ref-id>
adherence:
  # Global default: applies to all plans unless overridden
  evaluation-methods:
    - id: default-eval
      type: Behavioral
      mode: Automated
      executor:
        id: opa
        name: OPA/Conftest
        type: Software
        version: "0.1.0"
  assessment-plans:
    # Automated plan — inherits global evaluation-methods and executor
    - id: <plan-id>
      requirement-id: "<req-id>"
      frequency: "<cadence>"
      evidence-requirements: "<what>"
      parameters:
        - id: <param-id>
          label: "<label>"
          accepted-values: ["<value>"]
          description: "<rationale>"
    # Manual plan — overrides global with per-plan evaluation-methods
    - id: <plan-id-manual>
      requirement-id: "<req-id-manual>"
      frequency: "<cadence>"
      evaluation-methods:
        - id: manual-review
          type: Intent
          mode: Manual
      evidence-requirements: "<what>"
    # Plan with different executor — overrides global executor only
    - id: <plan-id-custom>
      requirement-id: "<req-id-custom>"
      frequency: "<cadence>"
      evaluation-methods:
        - id: custom-eval
          type: Behavioral
          mode: Automated
          executor:
            id: custom-scanner
            name: Custom Scanner
            type: Software
            version: "2.0.0"
      evidence-requirements: "<what>"
```

Use `scope.in.groups` for applicability groups from the scoping stage (e.g., maturity levels, risk tiers). This scopes the policy to only the controls that match those groups.

### Step 6: Validate

Write `.complytime/child-policy.yaml`. If the Gemara MCP server is available, validate against the schema with the Policy definition.

### Step 7: Present Summary

Show mapping references, imported catalogs/guidance, and the automation split:

```
Policy written to .complytime/child-policy.yaml

Assessment plans: 12 total
  Automated (evaluator: opa): 8 requirements
  Manual: 4 requirements

Invoke /comply:pack to generate assessment logic for the 8 automated requirements.
```

## MCP Resources and Tools

- `complypack://catalog/*` — Control Catalogs, Guidance Catalogs, Policies
- `get_assessment_requirements` — get requirement details and parameters for building assessment plans

**DO NOT parse local YAML files to extract control data.** Use `get_assessment_requirements` to get requirement text, parameters, and applicability. All control data MUST come from MCP resources or tools.

## Red Flags

- [ ] Any unresolved parameters?
- [ ] Every mapping reference has a corresponding import?
- [ ] Every assessment plan has frequency and evaluation method?
- [ ] Every value came from MCP or the delta report?
- [ ] Did you use `get_assessment_requirements`, not parse files?
- [ ] Did you check each requirement's applicability for automation eligibility?
- [ ] Does every automated plan have an executor with a named trusted evaluator?
- [ ] Did you ask the user to identify the trusted evaluator?
- [ ] Did you present the triage results and ask if any plans need a different evaluator or mode?
