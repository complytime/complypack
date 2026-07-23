# ADR 020: Coverage and Gap Report Engine

**Status:** Accepted

**Date:** 2026-07-23

## Context

After generating Rego policies for in-scope controls, consumers have no way
to see which controls have enforcement and which don't. Answering "Of the 14
controls in my Policy, how many have policies, how many pass, and which are
gaps?" today requires manually comparing the Policy's requirements against
files in the `policy/` directory. This does not scale, falls out of sync
immediately, and has been requested by both users and stakeholders. There is
also a need for CI/CD integration — automated coverage checks in pipelines
to catch gaps before merge.

The coverage report compares a Policy's in-scope controls against existing
enforcement artifacts and (optionally) test/evaluation results, producing a
structured summary with three buckets:

- **Implemented, passing** — enforcement artifact exists, tests pass
- **Implemented, failing** — enforcement artifact exists, tests fail
  (remediation needed)
- **Not implemented (gap)** — no enforcement artifact exists (build or
  accept risk)

A key design question is how the system determines a control is
"implemented." Options include:

1. Presence of a file matching the control ID pattern (e.g., via
   `complytime-mapping.json`)
2. An existing EvaluationLog entry
3. Explicit declaration in the Policy

The chosen approach is **hybrid**: use the evaluator's mapping file (e.g.,
`complytime-mapping.json` for OPA) as the primary signal for implementation
detection, with optional enrichment from EvaluationLog data when available
to determine pass/fail status.

## Decision

Build a compiled, domain-level coverage engine as a standalone type in a new
`internal/coverage/` package (or within `internal/requirement/`). The engine
is evaluator-agnostic from day one — it queries evaluator metadata (file
extension, required files) without modifying the `Evaluator` interface. It
is exposed through both transports per ADR-019 (CLI/MCP parity):

- **CLI**: `complypack coverage` command with human-readable default output
  and `--output json` for machine consumption
- **MCP**: `get_coverage_report` tool returning structured JSON

Inputs:

- Policy artifact (or effective policy name from MCP server)
- Policy directory path (where enforcement files live)
- Optionally: existing EvaluationLog to determine pass/fail status

The coverage engine:

1. Resolves the effective policy to extract in-scope control IDs (reusing
   `internal/requirement/` policy resolution)
2. Scans the policy directory using the evaluator's mapping file as the
   primary detection signal, falling back to file-extension-based directory
   scanning
3. If an EvaluationLog is provided, enriches implementation status with
   pass/fail results
4. Produces a structured report with per-control status and aggregate
   coverage metrics

## Alternatives Considered

### Alternative 1: Agent-Guided Gap Analysis via SKILL.md

A SKILL.md file instructs the LLM agent to list Policy requirements, scan
the `policy/` directory, and report gaps in natural language.

**Why considered:** Zero compiled code, rapid iteration, matches the
existing skill-based workflow (e.g., `build-assessment`).

**Why rejected:** Non-deterministic — depends on agent file-system
traversal accuracy and model capabilities. Cannot be used in CI/CD
pipelines without an LLM in the loop. Different models may produce
different results for the same inputs, making it unsuitable for automated
compliance verification.

### Alternative 2: CI-Only Coverage Report

A `complypack coverage` CLI command that runs exclusively in pipelines,
with no MCP integration.

**Why considered:** Simpler scope, directly addresses the CI/CD need
without touching the MCP server.

**Why rejected:** Violates ADR-019's parity guideline without good reason.
The coverage report is equally valuable in the MCP-assisted authoring
workflow — practitioners generating policies with `build-assessment` need
real-time visibility into which controls still need enforcement.

### Alternative 3: Manual Tracking

Maintain a spreadsheet or checklist mapping controls to policy files.

**Why considered:** No engineering effort required.

**Why rejected:** Does not scale. Falls out of sync as policies are added,
modified, or deleted. Offers no integration with the existing toolchain.

## Consequences

### Positive

- Deterministic, reproducible coverage reports usable in both interactive
  (MCP) and automated (CI/CD) workflows
- Evaluator-agnostic from day one — new policy languages get coverage
  reporting by implementing the existing `Evaluator` interface without
  additional work
- Follows ADR-019's domain-first architecture: domain logic in
  `internal/`, thin wiring in both transports
- Mapping-file-based detection is precise and avoids false positives from
  filename heuristics
- Reuses existing policy resolution infrastructure
  (`internal/requirement/`)

### Negative

- Adds a new domain package and two transport bindings (CLI command + MCP
  tool) — moderate implementation effort
- Mapping-file dependency means coverage detection requires evaluators to
  provide a mapping file; evaluators without one fall back to less precise
  directory scanning
- EvaluationLog enrichment is optional, so the report may show
  "implemented" without confirming tests actually pass unless the log is
  provided

### Risks

- If the Gemara framework adds native coverage tracking upstream, this
  engine may become redundant — revisit if that happens
- If ComplyPack moves away from file-based policy artifacts to a different
  enforcement model (e.g., inline policies, remote evaluation), the
  file-scanning approach would need rearchitecting

## Related Decisions

- [ADR 018: Test-Driven Rego Policy Generation](018-test-driven-rego-generation.md) — relates to: defines the policy+test generation workflow whose outputs this report measures
- [ADR 019: CLI and MCP Transport Parity](019-cli-mcp-parity.md) — constrains: mandates domain-first architecture with dual transport exposure
