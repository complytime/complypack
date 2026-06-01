# ADR 004: Fail-Fast Server Startup

**Date:** 2026-05-30  
**Status:** Accepted  
**Deciders:** Jennifer Power, Claude

## Context

The MCP server needs to pull catalogs from OCI registries, load schemas, and validate configuration at startup. We need to decide how to handle errors during this initialization phase.

## Decision

**Fail-fast: Refuse to start if any critical initialization step fails.**

The MCP server will:
- Exit with error if `complypack.yaml` is missing or invalid
- Exit with error if any catalog pull fails
- Exit with error if any user-provided schema fails to load
- Exit with error if configuration is inconsistent (e.g., platform not found)

**Never start in a degraded state with partial data.**

## Rationale

### Why fail-fast?

1. **Correctness over availability:** Better to fail than provide incorrect/incomplete data to LLM
2. **Clear error messages:** User gets immediate, specific feedback about what's wrong
3. **Predictable behavior:** Server is either fully functional or not running
4. **Prevents subtle bugs:** LLM won't generate policies based on incomplete catalog data

### Why not graceful degradation?

**Graceful degradation** would mean:
- Start server even if some catalogs fail to pull
- Serve partial data and log warnings
- Allow some resources to return "not available" errors

**Problems with graceful degradation:**
- LLM might generate incomplete policies based on partial catalog
- Hard to debug: "Why isn't control AC-5 showing up?"
- Silent failures: User doesn't realize data is missing
- Complex state: Server in half-working state

### Considered Alternatives

**Start with empty state, lazy-load on demand:**
- ❌ First resource request has high latency (pulling catalog)
- ❌ Errors surface during LLM interaction, not startup
- ✅ Faster startup time

**Start with cached data, refresh in background:**
- ❌ Stale data if cache is old
- ❌ Complex cache invalidation logic
- ✅ Fast startup with cached data
- ✅ Eventually consistent

## Fail-Fast Error Cases

| Error Condition | Behavior | Rationale |
|----------------|----------|-----------|
| No `complypack.yaml` found | Exit with error | Can't know what catalogs/schemas to load |
| Invalid `complypack.yaml` syntax | Exit with error | Configuration is malformed |
| Missing required field (`platform`) | Exit with error | Don't know target platform |
| Unsupported platform | Exit with error | Can't generate policies for unknown platform |
| Catalog OCI pull failure | Exit with error | Would have incomplete catalog data |
| Duplicate catalog IDs | Exit with error | Ambiguous resource URIs |
| User schema file not found | Exit with error | Configuration points to missing file |
| User schema invalid CUE | Exit with error | Can't parse user's schema |
| User schema remote fetch failure | Exit with error | Network/auth issue needs fixing |

## Graceful Degradation Cases

Some cases are warnings, not errors:

| Condition | Behavior | Rationale |
|-----------|----------|-----------|
| Catalog missing `metadata.id` | Log warning, use fallback name | Can infer name from OCI reference |
| User schema overrides built-in | Log info | Intentional override |

## Error Messages

All errors include:
- What went wrong
- Why it's a problem
- How to fix it

**Examples:**

```
Error: No complypack.yaml found in current directory.

Create complypack.yaml with:
  platform: kubernetes
  gemara-catalogs:
    - oci://ghcr.io/complytime/controls-catalog:v1
```

```
Error: Failed to pull catalog from oci://ghcr.io/complytime/controls-catalog:v1
  Cause: authentication failed

Fix: Run 'docker login ghcr.io' and try again.
```

```
Error: Platform 'foobar' not found.

Available platforms: kubernetes, terraform, docker, ansible, ci, custom-platform

Check your complypack.yaml 'platform' field.
```

## Trade-offs Accepted

- **Less availability:** Server won't start in degraded state
- **Network dependency:** Startup fails if OCI registry unreachable
- **Strict validation:** Minor configuration errors prevent startup

## Benefits

- **Predictable state:** Server is either fully functional or not running
- **Clear errors:** Immediate feedback on what's wrong
- **Data integrity:** LLM always gets complete, correct data
- **Simpler code:** No complex partial-state handling

## Consequences

### Positive

- Clear, actionable error messages
- No silent failures or partial data
- Easy to debug: server either works or doesn't
- User gets fast feedback on configuration issues

### Negative

- Server won't start if network is down (can't pull catalogs)
- Transient errors (temporary network issue) prevent startup
- No offline mode or cached fallback

## Future Considerations

**Potential enhancements (not in initial scope):**

1. **Offline mode:** Use cached catalogs if network unavailable
   - Requires persistent cache with metadata
   - `--offline` flag to skip OCI pulls

2. **Pre-flight check:** Validate configuration without starting server
   - `complypack mcp check` command
   - Validates config and checks OCI connectivity

3. **Retry logic:** Retry failed OCI pulls with exponential backoff
   - Currently: fail immediately
   - Future: retry 3 times before failing

## Implementation

1. Startup sequence validates at each step
2. First error encountered: exit immediately
3. Error messages include root cause and remediation
4. Exit code: 1 for all startup failures
5. Logging: Use `slog.Error()` before exiting
