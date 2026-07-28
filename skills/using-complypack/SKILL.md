---
name: using-complypack
description: Use when starting any compliance task — establishes skill ordering and MCP grounding requirements before any other complypack skill runs
---

# Using ComplyPack Skills

## The Rule

**Check for applicable complypack skills before any compliance action** — including answering questions about controls, generating policy, or modifying pipeline artifacts.

## Available Skills

| Skill | Type | When |
|-------|------|------|
| `comply:setup` | Process | Before any other skill — configures complypack for the project |
| `comply:build-assessment` | Domain | Generating Rego policies from Gemara catalogs (`single` or `batch` mode) |
| `comply:audit-pipeline` | Domain | Building Gemara Policy artifacts (scoping, mapping, adherence) |

### Internal Skills (not user-invocable)

| Skill | Used By |
|-------|---------|
| `comply:test-driven-assessment` | `comply:build-assessment` — test case generation logic |
| `comply:pack-assessment` | `comply:build-assessment` — policy generation logic |

## Skill Priority

- "Set up for compliance work" → `comply:setup` first (required once per project)
- "Generate a Rego policy" → `comply:build-assessment` (handles test-then-policy internally)
- "Run the comply pipeline" → `comply:audit-pipeline` directly (MCP must already be configured)

## MCP Grounding

**Every complypack skill reads from MCP.** Control IDs, requirement IDs, parameter values, and platform schemas come from MCP resources — never from model memory or training data.

If the MCP server is unreachable, **stop and inform the user.** Do not proceed with data from general knowledge. The skills will produce incorrect output without MCP grounding.

## Red Flags — STOP AND FIX IF THERE ARE ISSUES

- [ ] MCP server is unreachable → **STOP.** Do not fall back to general knowledge. Inform the user and wait. The output will look correct and be wrong.
- [ ] `comply:test-driven-assessment` or `comply:pack-assessment` invoked directly by user → **STOP.** These are internal skills. Use `comply:build-assessment` instead.
- [ ] Control IDs, requirement IDs, or parameter values came from model memory → **STOP.** Re-read from MCP. Training data may be outdated or wrong.
