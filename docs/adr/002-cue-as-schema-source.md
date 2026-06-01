# ADR 002: CUE as Schema Source of Truth

**Date:** 2026-05-30  
**Status:** Accepted  
**Deciders:** Jennifer Power

## Context

Platform input schemas define the structure of data that policies evaluate (Kubernetes manifests, Terraform plans, etc.). We need to decide what format to use as the source of truth and what format to expose to LLMs via the MCP server.

The `complypack-pipeline` project already has these schemas written in CUE.

## Decision

**Use CUE as the source of truth, generate JSON Schema for LLM consumption.**

```
schemas/
  cue/                          # Source of truth
    kubernetes.cue
    terraform.cue
    ...
  json-schema/                  # Generated for MCP
    kubernetes.json             # Generated via: cue export --out openapi
    terraform.json
    ...
```

## Rationale

### Why CUE as source?

1. **Already exists:** `complypack-pipeline` has schemas in CUE - we can reuse them
2. **More expressive:** CUE is more powerful than JSON Schema for validation and constraints
3. **Universal translator:** CUE can export to multiple formats:
   - JSON Schema (for LLMs)
   - OpenAPI (for API docs)
   - Go structs (for type-safe code)
   - Plain JSON (for runtime validation)
4. **Maintainability:** CUE is easier to read and maintain than verbose JSON Schema

### Why expose JSON Schema via MCP?

1. **LLM compatibility:** Claude and other LLMs understand JSON Schema natively
2. **No runtime CUE dependency:** Tools consuming schemas don't need CUE installed
3. **Broader ecosystem:** JSON Schema is universal, CUE is more niche

### Considered Alternatives

**Pure CUE (expose CUE via MCP):**
- ❌ LLMs don't understand CUE format natively
- ❌ Would require teaching LLM about CUE syntax
- ✅ No conversion/generation step needed

**Pure JSON Schema (no CUE):**
- ❌ More verbose and harder to maintain
- ❌ Can't leverage CUE's validation capabilities
- ❌ Would need to manually translate from complypack-pipeline's CUE
- ✅ Simpler toolchain (no CUE dependency)

**Runtime conversion (CUE → JSON Schema on-the-fly):**
- ❌ Adds latency at MCP server startup
- ❌ Requires CUE Go API at runtime
- ✅ Always up-to-date (no generated files to commit)

## Trade-offs Accepted

- **Build step required:** Must regenerate JSON Schema when CUE changes
- **Two copies in repo:** Both CUE source and JSON Schema committed
- **Potential drift:** JSON Schema can become stale if CUE changes without regeneration

## Mitigation

- Add `make generate-schemas` to regenerate JSON Schema from CUE
- Add CI check to verify JSON Schema is up-to-date with CUE source
- Document generation process in `schemas/README.md`

## Consequences

### Positive

- Reuse existing CUE schemas from complypack-pipeline
- CUE serves as the "universal translator" for future needs
- LLMs get JSON Schema they understand
- Can export to other formats in future (OpenAPI, Go types, etc.)

### Negative

- Requires CUE toolchain for development (but not for runtime)
- Need to remember to regenerate JSON Schema after CUE edits
- Two files to maintain per platform

## Implementation

1. Copy CUE schemas from `complypack-pipeline/schemas/*.cue` to `schemas/cue/`
2. Create `Taskfile.yml` with `generate-schemas` task
3. Generate initial JSON Schema files via `cue export --out openapi`
4. Commit both CUE (source) and JSON Schema (generated) to repo
5. Binary embeds JSON Schema via `embed.FS`
6. Add CI check to verify JSON Schema is in sync with CUE

## Future Considerations

- Could generate Go structs from CUE for type-safe policy validation
- Could generate OpenAPI specs for documenting platform schemas
- Could use CUE at runtime for advanced validation (not just JSON Schema)
