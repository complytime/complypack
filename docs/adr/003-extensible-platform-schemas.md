# ADR 003: Extensible Platform Schemas

**Date:** 2026-05-30  
**Status:** Superseded by [ADR 017](017-schema-index.md)  
**Deciders:** Jennifer Power

## Context

The initial design hardcoded support for five platforms (kubernetes, terraform, docker, ansible, ci). This is too restrictive - users may need to generate policies for custom platforms or infrastructure not covered by the built-ins.

## Decision

**Support both built-in platforms and user-provided custom platforms via configuration.**

Built-in schemas are embedded in the binary. Users can extend with custom schemas via `complypack.yaml`:

```yaml
platform: custom-platform
gemara-catalogs:
  - oci://ghcr.io/complytime/controls-catalog:v1
platform-schemas:
  custom-platform: ./schemas/custom.cue
  special-infra: https://example.com/schemas/platform.cue
```

## Rationale

### Why extensibility?

1. **Unpredictable platform needs:** Can't predict every platform users will target
2. **Custom infrastructure:** Organizations have custom platforms we can't know about
3. **Rapid platform evolution:** New infrastructure tools emerge constantly
4. **Override capability:** Users may want to customize built-in schemas

### Why configuration-based extension?

1. **Explicit and clear:** User declares exactly what schemas are available
2. **Version control friendly:** `complypack.yaml` lives in the repo
3. **Supports local and remote:** Can use local files or fetch from URLs
4. **No directory scanning:** Explicit is better than implicit discovery

### Considered Alternatives

**Hardcoded platforms only:**
- ❌ Too restrictive
- ❌ Requires complypack updates for new platforms
- ✅ Simpler implementation
- ✅ Fewer edge cases

**Auto-discovery from directory:**
```
.complypack/schemas/
  custom-platform.cue
  another-platform.cue
```
- ❌ Implicit behavior (magic directory)
- ❌ Harder to debug ("why isn't my schema loading?")
- ✅ No configuration needed
- ✅ Just drop files and go

**Plugin system:**
- ❌ Over-engineered for this need
- ❌ Adds significant complexity
- ✅ Most flexible approach

## Design

### Schema Loading Priority

1. Load all built-in schemas (from embedded files)
2. Load user-provided schemas (from `platform-schemas` config)
3. Merge: **user schemas override built-ins** if names conflict
4. Validate that `platform` exists in merged set

### User Schema Sources

**Local CUE files:**
```yaml
platform-schemas:
  custom: ./schemas/custom.cue
```
- Relative to `complypack.yaml` location
- Converted from CUE → JSON Schema at MCP startup

**Remote URLs:**
```yaml
platform-schemas:
  shared: https://example.com/schemas/platform.cue
```
- Fetched at MCP startup
- Cached (TBD: cache location and invalidation)

### Override Behavior

If user provides a schema with same name as built-in:

```yaml
platform-schemas:
  kubernetes: ./schemas/custom-k8s.cue
```

- User's `custom-k8s.cue` **replaces** the built-in `kubernetes` schema
- Logged: "Using custom schema for platform 'kubernetes'"

## Trade-offs Accepted

- **Startup latency:** Fetching remote schemas adds delay
- **Network dependency:** Remote schemas require network access
- **CUE runtime dependency:** Need CUE Go API to convert user schemas
- **More error cases:** Invalid CUE, unreachable URLs, parse failures

## Error Handling

| Error | Behavior |
|-------|----------|
| User schema file not found | Refuse to start: "Schema file './schemas/custom.cue' not found" |
| User schema invalid CUE | Refuse to start: "Failed to parse schema 'custom': <CUE error>" |
| User schema remote fetch failure | Refuse to start: "Failed to fetch schema from https://...: <error>" |
| User schema overrides built-in | Log warning: "Custom schema overrides built-in platform 'kubernetes'" |

**Fail-fast:** Server refuses to start with invalid/unreachable user schemas.

## Consequences

### Positive

- Users can support any platform
- Organizations can share schemas via URLs
- Built-in schemas provide good defaults
- Override mechanism allows customization

### Negative

- More complex schema loading logic
- CUE Go API required at runtime (for user schemas)
- Network dependency for remote schemas
- More error cases to handle

## Implementation

1. Extend `complypack.yaml` schema to include `platform-schemas` map
2. Implement schema loading:
   - Load built-ins from `embed.FS`
   - Load user CUE from local files
   - Fetch user CUE from URLs
   - Convert all CUE → JSON Schema
   - Merge (user overrides built-ins)
3. Update `ListResources` to include all (built-in + user) schemas
4. Update `ReadResource` to serve from merged map
5. Update error messages to list available platforms

## Future Considerations

- **Schema caching:** Cache remote schemas locally with TTL
- **Schema validation:** Validate user schemas against a meta-schema
- **Schema registry:** Central registry of community-contributed schemas
- **Hot reload:** Reload schemas without restarting MCP server
