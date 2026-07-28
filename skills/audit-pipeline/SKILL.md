---
name: audit-pipeline
description: Use when user wants to build Gemara Policy artifacts for audit preparation or compliance program setup
---

# /comply:audit-pipeline — ComplyTime Audit Pipeline

Guide users through building a Gemara Policy (applicability statement) from their system architecture and governance sources. The Gemara Policy is the formal contract between audit and engineering, functionally equivalent to an ISO 27001 Statement of Applicability or a NIST System Security Plan.

## Safety

**CRITICAL:** Every stage MUST read control IDs, requirement IDs, and parameter values from MCP resources. DO NOT generate these from memory. The MCP server is the source of truth.

## Pipeline Stages

| Stage     | Artifact                          | Purpose                                                          |
|-----------|-----------------------------------|------------------------------------------------------------------|
| scoping   | `.complytime/scoping.yaml`        | System profile + Control Catalog scoping + gap analysis          |
| mapping   | `.complytime/delta-report.yaml`   | Parameter delta analysis + harmonization across framework layers |
| adherence | `.complytime/child-policy.yaml`   | Compile the child Policy with adherence plan                     |

After adherence, invoke `/comply:build-assessment` to generate assessment logic for use at runtime.

## Router Logic

1. Check if `.complytime/` directory exists and which artifacts are present
2. Determine pipeline state:
   - No `.complytime/` directory → start at **scoping**
   - `scoping.yaml` exists but no `delta-report.yaml` → offer **mapping**
   - `delta-report.yaml` exists but no `child-policy.yaml` → offer **adherence**
   - `child-policy.yaml` exists → pipeline complete, offer to re-run any stage or proceed to `/comply:build-assessment`
3. If the user specified a stage, validate prerequisites:
   - **mapping** requires `scoping.yaml`
   - **adherence** requires `delta-report.yaml`
4. Dispatch to the appropriate stage skill

## Dispatching

Read the stage instructions from this skill's base directory before proceeding:

- **scoping** → `scoping.md`
- **mapping** → `mapping.md`
- **adherence** → `adherence.md`

## Supporting Files

- **policy-validation-rubric.md** — Structural validation rubric for compiled Policy YAML (consumed by Step 6a of adherence)

## Status Display

```text
/comply:audit-pipeline status:
  [done] scoping     — .complytime/scoping.yaml
  [done] mapping     — .complytime/delta-report.yaml
  [next] adherence   — not yet run
```
