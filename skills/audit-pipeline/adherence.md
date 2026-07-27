---
name: adherence
description: Populate a Gemara Policy defining what controls apply, with what parameter values, and how evidence will be collected
user-invocable: false
---

# Adherence — Compile Policy

Compile or alter a Gemara Policy artifact. Declares what controls apply, with exact parameter values, and defines the adherence plan: frequency, evaluation method, and evidence requirements.

Evidence collection and Evaluation Logs are produced at runtime by a compliant evaluation engine.

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

> **Critical:** Set `mapping-reference.id` to match the referenced
> artifact's `metadata.id` exactly (e.g., `container-security-controls`,
> not `csc`). ComplyPack resolves policy imports by matching these IDs
> against loaded sources. Short aliases pass Gemara schema validation but
> fail at complypack runtime with:
> `failed to resolve effective policy: no imports could be resolved`
>
> DO NOT include a `url` field in mapping references. ComplyPack resolves
> artifacts by `id` alone and local `file://` paths become broken
> references after OCI packaging.

### Step 3: Build Imports

- `imports.catalogs` — one per Control Catalog
- `imports.guidance` — one per mandated Guidance Catalog

### Step 4: Build Assessment Plans

#### 4a: Identify the trusted evaluator

Ask the user: "Who is the trusted evaluator for automated assessments?" Read `complypack://evaluator` to see available evaluators. Collect:
- `id` — evaluator plugin identifier
- `name` — display name
- `type` — `Software`
- `version` — evaluator version

This becomes the **default** executor for all automated plans, set as a global evaluation method on `adherence.evaluation-methods`. Individual assessment plans can override this with their own `evaluation-methods` and `executor` when a specific requirement needs a different evaluator or mode.

> **Runtime requirement — `executor` field:**
> A runtime uses the `executor` field to route automated assessment plans to the correct policy engine. Every evaluation method with `mode: Automated` **must** include an `executor` block (with at minimum `id` and `name`). Without it, the runtime cannot connect the assessment plan to its evaluator and the plan will fail at runtime. Common executors include `opa` (Open Policy Agent) and `ampel`. Manual plans do not need an executor.

#### 4b: Triage requirements by automation eligibility

Call `get_applicability_groups` to retrieve the catalog's applicability group definitions and see which requirements belong to each group. Use the group titles and descriptions to identify which group designates automation-eligible requirements.

- **Automatable** (requirement belongs to the automation-eligible group): assessment plan inherits the global `mode: Automated` and executor — no per-plan `evaluation-methods` needed unless overriding
- **Not automatable**: assessment plan overrides with per-plan `evaluation-methods` set to `mode: Manual`, no executor
- **No applicability groups in catalog**: ask the user to classify each requirement as automated or manual

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
        id: <evaluator-id>
        name: <evaluator-name>
        type: Software
        version: "<evaluator-version>"
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
    # Plan with different executor — overrides global with per-plan executor
    - id: <plan-id-custom>
      requirement-id: "<req-id-custom>"
      frequency: "<cadence>"
      evaluation-methods:
        - id: custom-eval
          type: Behavioral
          mode: Automated
          executor:
            id: <other-evaluator-id>
            name: <other-evaluator-name>
            type: Software
            version: "<other-evaluator-version>"
      evidence-requirements: "<what>"
```

Use `scope.in.groups` for applicability groups from the scoping stage (e.g., maturity levels, risk tiers). This scopes the policy to only the controls that match those groups.

### Step 6: Validate

Write `.complytime/child-policy.yaml`, then run both validation phases below.

#### Step 6a: Structural validation (rubric)

Spawn a context-isolated subagent to validate the compiled Policy against
the rubric in `policy-validation-rubric.md`. The subagent receives ONLY
the contents of `.complytime/child-policy.yaml` and the rubric — no delta
report, no MCP resources, and no conversation history.

If the host does not support context-isolated subagent dispatch, perform
the rubric checks inline and emit a warning that context isolation was not
achieved. Inline execution is a degraded mode — the checks still run, but
the isolation guarantee against `url` hallucination from conversation
context does not hold.

If the subagent returns ALL CHECKS PASSED, proceed to Step 6b.

If the subagent returns any FAIL findings, fix every issue in the Policy
YAML and re-run Step 6a. Re-run a maximum of 3 times. If failures persist
after 3 attempts, present the remaining failures to the user and request
manual correction (per AGENTS.md: "Third failure: STOP AND ASK").

#### Step 6b: Schema validation (optional)

If the Gemara MCP server is available, validate `.complytime/child-policy.yaml`
against the schema with the Policy definition.

### Step 7: Present Summary

Show mapping references, imported catalogs/guidance, and the automation split:

```text
Policy written to .complytime/child-policy.yaml

Assessment plans: 12 total
  Automated (evaluator: <evaluator-name>): 8 requirements
  Manual: 4 requirements

Invoke /comply:build-assessment to generate assessment logic for the 8 automated requirements.
```

## MCP Resources and Tools

- `complypack://catalog/*` — Control Catalogs, Guidance Catalogs, Policies
- `get_assessment_requirements` — get requirement details and parameters for building assessment plans
- `get_applicability_groups` — get applicability group definitions and requirement memberships for automation triage

**DO NOT parse local YAML files to extract control data.** Use `get_assessment_requirements` and `get_applicability_groups` for requirement data and group classification. All control data MUST come from MCP resources or tools.

## Red Flags

- [ ] Any unresolved parameters?
- [ ] Every mapping reference has a corresponding import?
- [ ] Every assessment plan has frequency and evaluation method?
- [ ] Every value came from MCP or the delta report?
- [ ] Did you use `get_assessment_requirements`, not parse files?
- [ ] Did you use `get_applicability_groups` to check automation eligibility?
- [ ] Does `adherence.evaluation-methods` have an executor with the user-identified trusted evaluator?
- [ ] Did you ask the user to identify the trusted evaluator?
- [ ] Did you present the triage results and ask if any plans need a different evaluator or mode?
- [ ] Any mapping reference includes a `url` field? Remove it.
