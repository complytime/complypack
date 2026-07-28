# ComplyPack

ComplyPack provides AI coding assistants with compliance pipeline skills
and an MCP server for working with [Gemara](https://github.com/gemaraproj)
control catalogs.

## MCP Server

The complypack MCP server serves Gemara artifacts (catalogs, policies,
mappings) and provides tools for policy validation, assessment triage,
and parameter analysis. Configure it via `/comply-setup` or manually
in your tool's MCP configuration. See `mcps.json` for the base template.

## Skills

| Skill | Trigger | Purpose |
|-------|---------|---------|
| `using-complypack` | Starting any compliance task | Establishes skill ordering and MCP grounding requirements |
| `setup` | User wants to set up complypack | Interactive setup: generates complypack.yaml, warms cache, optional MCP config |
| `audit-pipeline` | User wants to build compliance artifacts or prepare for audit | Three-stage pipeline: scoping, mapping, adherence |
| `build-assessment` | User mentions Rego, Conftest, OPA, or policy generation | Test-driven Rego policy generation from Gemara catalogs |
| `test-driven-assessment` | Internal (invoked by build-assessment) | Generates human-reviewable test cases before policy is written |

### Pipeline Flow

```
scoping -> mapping -> adherence -> build-assessment
```

1. **Scoping** -- Characterize the system, scope control catalogs, identify gaps
2. **Mapping** -- Crosswalk frameworks, harmonize parameters across layers
3. **Adherence** -- Compile a Gemara Policy with assessment plans
4. **Build Assessment** -- Generate test cases, then Rego policies for automated assessment plans

## Commands

| Command | Description |
|---------|-------------|
| `/comply-setup` | Configure complypack MCP server for this project |
| `/comply-pipeline` | Run the comply pipeline (scoping, mapping, adherence) |
| `/comply-pack` | Generate test-driven Rego policies (single or batch mode) |

## Safety

All control IDs, requirement IDs, and parameter values MUST come from MCP
resources. Skills enforce this constraint to prevent hallucinated policy
content. The MCP server is the single source of truth.
