# ADR 008: Pre-Pack Validation Gates and Schema Path Unification

**Status:** Accepted

**Date:** 2026-06-03

**Context:**

Two architectural issues required a combined fix:

1. **Pack shipped unvalidated content.** `complypack pack` tarballed policy content and pushed to an OCI registry with zero quality checks. Broken Rego, invalid schema references, and failing tests all got published. For a compliance tool, this undermines trust in the artifact.

2. **Schema path divergence.** The MCP server loaded schemas from user-configured
   sources (CUE registry, HTTPS, file) and served them via
   `complypack://schema/<platform>` resources. However, the `validate_policy`
   MCP tool loaded CUE schemas exclusively from embedded files via
   `loadCUESchemaForPlatform()`, ignoring user configuration. If a user overrode
   a schema via config, the LLM saw one schema but contract validation ran
   against another. Additionally, `loadSchemas()` silently fell back to embedded
   schemas when a configured source failed, violating ADR 004 (fail-fast).

**Decision:**

## Part 1: Unified Schema Store

Extend `ResourceStore` to hold both `[]byte` (for MCP resource serving) and compiled `cue.Value` (for contract validation) per platform. Both representations are loaded once from the same configured source at startup.

- `loadSchemas()` returns `(map[string][]byte, map[string]cue.Value, error)`
- `ResourceStore.CUESchema(platform)` provides access to compiled schemas
- `handleValidatePolicy` reads from the store instead of loading embedded schemas independently
- Silent fallback removed: if a user-configured source fails, the server refuses to start (per ADR 004)
- Embedded schemas are used only when no source is configured

## Part 2: Pre-Pack Validation Pipeline

`complypack pack` runs a 3-stage validation pipeline before packaging:

1. **Syntax check** -- Parse all policy files matching the evaluator's extension
2. **Contract check** -- Verify `input.*` references against the CUE schema
3. **Test execution** -- Run policy unit tests

Each stage is fail-fast: syntax failure skips contract/test, contract failure skips tests.

CLI flags:

| Flag                 | Default | Behavior                                      |
| -------------------- | ------- | --------------------------------------------- |
| `--skip-validation`  | `false` | Skip all pre-pack validation                  |
| `--skip-tests`       | `false` | Run syntax + contract but skip test execution |

The pack command uses the same schema loading path (`LoadCUEFromSource`, `loadEmbeddedCUESchema`) as the MCP server.

**Consequences:**

**Benefits:**

- Single schema path: MCP resources, MCP tools, and pack validation all use the same loaded schemas
- No silent fallback: configuration errors are surfaced immediately
- Policies are validated before distribution: syntax, contract, and tests must pass
- Escape hatch: `--skip-validation` preserves backward compatibility for CI pipelines that validate separately

**Drawbacks:**

- Slower pack: validation adds latency (mitigated by OPA SDK in-process performance)
- Behavioral change: users with misconfigured schema sources that previously fell back silently to embedded will now see startup errors
- Pack requires evaluator to be registered (currently only OPA)

**Related:**

- ADR 004: Fail-Fast Server Startup
- ADR 005: Evaluator Interface Pattern
- ADR 006: CUE Schema Contract Validation
- ADR 007: OPA SDK In-Process
