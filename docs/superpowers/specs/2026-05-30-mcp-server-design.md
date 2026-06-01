# ComplyPack MCP Server Design

**Date:** 2026-05-30  
**Status:** Approved  
**Issue:** [#9 - Create MCP Server](https://github.com/complytime/complypack/issues/9)

## Overview

Implement an MCP (Model Context Protocol) server as a subcommand of the `complypack` CLI to enable LLM-assisted Rego policy generation. The MCP server exposes Gemara control catalogs and platform input schemas as resources that Claude (or other LLMs) can read when generating policies.

## Goals

1. Expose Gemara control catalogs (assessment requirements) as MCP resources
2. Expose platform input schemas (data structure definitions) as MCP resources
3. Enable LLMs to generate policies grounded in actual catalog requirements and valid platform fields
4. Support both built-in platforms and user-defined custom platforms

## Non-Goals

- Policy generation tools (deferred - will be added after CLI `validate` and `test` commands exist)
- Policy validation/testing via MCP (deferred)
- Workspace awareness beyond catalogs and schemas

## Architecture

### Command Structure

MCP server runs as a subcommand of the existing `complypack` CLI:

```bash
complypack mcp serve
```

**Rationale:** Single binary approach. Unlike gemara-mcp (which is ONLY an MCP server), complypack already has a CLI. Adding MCP as a subcommand keeps everything in one binary and shares code between CLI and MCP server.

### Directory Structure

```
schemas/
  cue/                          # Source of truth (CUE schemas)
    kubernetes.cue
    terraform.cue
    docker.cue
    ansible.cue
    ci.cue
  json-schema/                  # Generated for MCP consumption
    kubernetes.json
    terraform.json
    docker.json
    ansible.json
    ci.json
  Makefile                      # Schema generation tooling

cmd/complypack/cli/
  mcp.go                        # MCP serve command

internal/mcp/
  server.go                     # Server initialization & registration
  resources.go                  # Resource handlers
  config.go                     # complypack.yaml parsing
  consts.go                     # URI constants and metadata
  server_test.go                # Tests
  
internal/config/
  complypack.go                 # Shared ComplyPackConfig type
```

## Configuration

### complypack.yaml

Users create a `complypack.yaml` in their workspace:

```yaml
platform: kubernetes
gemara-catalogs:
  - oci://ghcr.io/complytime/controls-catalog:v1
  - oci://ghcr.io/complytime/security-controls:v2
platform-schemas:
  custom-platform: ./schemas/custom.cue
  special-infra: https://example.com/schemas/platform.cue
```

**Fields:**
- `platform` (required): Target platform for policy generation
- `gemara-catalogs` (required): List of OCI references to Gemara control catalogs
- `platform-schemas` (optional): Custom platform schemas (extends built-ins)

## Server Startup Flow

1. Read `complypack.yaml` from current working directory
2. **Pull Gemara catalogs:**
   - For each OCI reference in `gemara-catalogs`
   - Use existing `internal/registry` logic from `catalog pull` command
   - Download to `~/.complypack/cache/<hash>/catalog.yaml`
   - Parse catalog to extract `metadata.id` (e.g., "controls-v1")
   - Fallback to inferring name from OCI reference if `metadata.id` missing
3. **Load platform schemas:**
   - Load built-in schemas from embedded `schemas/json-schema/*.json`
   - Load user-provided schemas from `platform-schemas` config
   - Convert CUE → JSON Schema at startup (using CUE Go API)
   - Merge (user schemas can override built-ins)
4. **Validate configuration:**
   - Ensure `platform` exists in (built-ins + user-provided)
   - Detect duplicate catalog IDs
5. Register all resources with MCP server
6. Start stdio transport

## Resources Exposed

### Catalog Resources

**URI Pattern:** `complypack://catalog/<name>`

**Examples:**
- `complypack://catalog/controls-v1`
- `complypack://catalog/security-v2`

**Content:**
- MIMEType: `application/yaml`
- Body: Full Gemara catalog YAML content
- Source: Pulled from OCI registry and cached locally

**Catalog Name Resolution:**
1. Try `metadata.id` from catalog
2. Fallback: Extract from OCI reference (`controls-catalog:v1` → `controls-v1`)
3. Log warning if using fallback

### Platform Schema Resources

**URI Pattern:** `complypack://schema/<platform>`

**Examples:**
- `complypack://schema/kubernetes`
- `complypack://schema/terraform`
- `complypack://schema/custom-platform` (user-provided)

**Content:**
- MIMEType: `application/json`
- Body: JSON Schema describing platform input structure
- Source: Built-in (embedded) or user-provided (from config)

**Built-in Platforms:**
- `kubernetes` - Kubernetes resources (Deployment, Pod, Service, etc.)
- `terraform` - Terraform plan JSON structure
- `docker` - Dockerfile and container config
- `ansible` - Ansible playbook structure
- `ci` - GitLab CI / GitHub Actions YAML

**User-Provided Platforms:**
- Defined in `complypack.yaml` under `platform-schemas`
- Can be local CUE files or remote URLs
- Override built-ins if names conflict

## MCP Resource Discovery

**ListResources:**
- Returns all available catalogs and schemas
- Example response:
  ```json
  {
    "resources": [
      {"uri": "complypack://catalog/controls-v1", "name": "ESS v11 Controls"},
      {"uri": "complypack://catalog/security-v2", "name": "ITSS Controls"},
      {"uri": "complypack://schema/kubernetes", "name": "Kubernetes Platform Schema"},
      {"uri": "complypack://schema/terraform", "name": "Terraform Platform Schema"},
      {"uri": "complypack://schema/custom-platform", "name": "Custom Platform Schema"}
    ]
  }
  ```

**ReadResource:**
- Returns content for a specific URI
- Errors if resource not found

## Schema Management

### CUE as Source of Truth

Platform schemas are authored in CUE (more expressive and maintainable) but exposed as JSON Schema (LLM-friendly).

**Generation Process:**
```bash
make generate-schemas
# For each schemas/cue/*.cue file:
#   cue export --out openapi <file> > schemas/json-schema/<name>.json
```

**Rationale:**
- CUE is the universal translator (can export to JSON Schema, OpenAPI, Go types, etc.)
- JSON Schema is what LLMs understand
- Commit both CUE (source) and JSON Schema (generated) to repo
- Binary embeds JSON Schema for runtime use

### Schema Sources

**Built-in schemas:**
- Copied from `complypack-pipeline/schemas/*.cue`
- Embedded in binary via `embed.FS`

**User-provided schemas:**
- Loaded from `platform-schemas` config at runtime
- Converted from CUE → JSON Schema using CUE Go API
- Support local files (`./schemas/custom.cue`) and remote URLs

## Error Handling

| Error Condition | Behavior | Message |
|----------------|----------|---------|
| Missing `complypack.yaml` | Refuse to start | "No complypack.yaml found in current directory. Create one with 'platform' and 'gemara-catalogs' fields." |
| Unsupported platform | Refuse to start | "Platform 'foo' not supported. Available platforms: [kubernetes, terraform, custom-platform, ...]" |
| Catalog pull failure | Refuse to start | "Failed to pull catalog from oci://...: <error>. Check network/authentication." |
| Duplicate catalog IDs | Refuse to start | "Duplicate catalog ID 'controls-v1' from oci://... and oci://..." |
| Missing catalog metadata | Log warning, use fallback | "Catalog missing metadata.id, using inferred name: controls-v1" |
| Resource not found (catalog) | Return MCP error | "Catalog 'unknown' not found. Available catalogs: [controls-v1, security-v2]" |
| Resource not found (schema) | Return MCP error | "Schema 'unknown' not found. Available platforms: [kubernetes, terraform, ...]" |
| Invalid user schema CUE | Refuse to start | "Failed to parse platform schema 'custom': <CUE error>" |

**Fail-Fast Philosophy:**
- Server refuses to start with incomplete/invalid configuration
- Better to fail early than serve partial data

## Testing Strategy

### Unit Tests

**Files:**
- `internal/mcp/server_test.go`
- `internal/mcp/resources_test.go`
- `internal/config/complypack_test.go`

**Test Cases:**
- Parse valid `complypack.yaml`
- Parse invalid config (missing fields, unknown platform)
- Catalog name extraction from metadata
- Catalog name fallback from OCI reference
- Duplicate catalog ID detection
- Resource listing returns correct URIs
- Resource reading returns correct content
- Schema embedding and retrieval
- User schema loading and merging
- User schema overriding built-ins

### Integration Tests

**Setup:**
- Create test `complypack.yaml`
- Use `memory.New()` OCI store with pre-packed test catalogs
- Mock user-provided schemas

**Verify:**
- Server starts successfully
- `ListResources` returns expected resources
- `ReadResource` returns correct catalog/schema content
- User schemas accessible
- User schemas override built-ins

### Manual Testing

1. Create `complypack.yaml` in test directory
2. Run `complypack mcp serve`
3. Connect via MCP inspector or Claude Code
4. Verify resources are listed
5. Verify resources can be read
6. Test with custom platform schema

## Dependencies

**New:**
- `github.com/modelcontextprotocol/go-sdk` - MCP server SDK
- `cuelang.org/go/cue` - CUE parsing and conversion (if not already present)

**Existing:**
- `internal/registry` - OCI catalog pulling (from `catalog pull` command)
- `github.com/spf13/cobra` - CLI framework

## Implementation Order

1. Add platform schemas (CUE source + JSON Schema generated) - addresses Issue #7
2. Implement `internal/config/complypack.go` - config parsing
3. Implement `internal/mcp/server.go` - server initialization
4. Implement `internal/mcp/resources.go` - catalog & schema resources
5. Implement `cmd/complypack/cli/mcp.go` - CLI command
6. Add tests
7. Documentation

## Future Enhancements

**Not in scope for this design, but noted for future:**

1. **MCP Tools** (after CLI commands exist):
   - `validate_policy` - Validate Rego syntax/style
   - `test_policy` - Run OPA tests against fixtures

2. **MCP Prompts** (similar to gemara-mcp):
   - Guided wizards for policy generation
   - Template-based policy creation

3. **Workspace Awareness**:
   - `complypack://config` - Current configuration
   - `complypack://policies` - List policies in workspace

4. **Catalog Versioning**:
   - `complypack://catalog/controls-v1?version=1.2.3`
   - Support multiple versions of same catalog

## Related ADRs

See `docs/adr/` for architectural decision records:
- `001-mcp-as-cli-subcommand.md`
- `002-cue-as-schema-source.md`
- `003-extensible-platform-schemas.md`
- `004-fail-fast-server-startup.md`
