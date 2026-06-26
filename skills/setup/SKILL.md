---
name: setup
description: Configure the Gemara and complypack MCP servers for this project — set up artifact sources, platform schemas, and generate the .mcp.json config
---

# /comply:setup — Configure MCP Servers

Set up the Gemara MCP server and the complypack MCP server for this project.

## MCP Servers

| Server         | Purpose                              | Provides                                                                                                                                                                    |
| -------------- | ------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **gemara**     | Authoring and validation             | `gemara://lexicon`, `gemara://schema/definitions`, `validate_gemara_artifact`                                                                                               |
| **complypack** | Artifact serving, policy validation  | `complypack://catalog/*`, `complypack://mapping/*`, `complypack://schema/*`, `validate_policy`, `test_policy`, `get_assessment_requirements`, `analyze_parameter_delta`       |

## Process

### Step 1: Check Existing Configuration

Check if `.mcp.json` already exists. Show current config and ask if the user wants to reconfigure.

### Step 2: Configure Sources

Ask for Gemara artifact sources for the complypack server:

- `oci://registry.example.com/gemara/controls:v1`
- `oci+http://localhost:5001/gemara/controls:v1` (development)
- `file://path/to/catalog.yaml`

At least one source is required.

### Step 3: Configure Schemas

Ask which platform schemas to load:

- `kubernetes` (embedded)
- `ci` (embedded)
- `ci=cue://cue.dev/x/githubactions@v0#Workflow` (CUE registry)

### Step 4: Resolve Versions

Look up latest release versions. Do NOT use `:latest` tags.

- **gemara-mcp**: `gh api repos/gemaraproj/gemara-mcp/releases/latest --jq '.tag_name'`
- **complypack**: `gh api repos/complytime/complypack/releases/latest --jq '.tag_name'`

If no release exists, ask the user for a version to pin.

### Step 5: Detect Tool Environment

Determine which AI coding tool is running and adapt the output:

- **Claude Code**: Check if `.claude-plugin/` exists in the repo. Write `.mcp.json`.
- **OpenCode**: Check if `.opencode/` exists in the repo. Write `.mcp.json`. Verify that `.opencode/skills/` symlinks exist — if not, create them:
  ```bash
  mkdir -p .opencode/skills
  ln -sf ../../skills/pipeline .opencode/skills/pipeline
  ln -sf ../../skills/pack .opencode/skills/pack
  ln -sf ../../skills/setup .opencode/skills/setup
  ```
- **Cursor**: Check if `.cursor-plugin/` exists. Write `.mcp.json`.
- **Unknown**: Write `.mcp.json` and inform the user about skill discovery.

### Step 6: Write Configuration

Write `.mcp.json`:

```json
{
  "mcpServers": {
    "gemara": {
      "command": "docker",
      "args": ["run", "--rm", "-i",
               "ghcr.io/gemaraproj/gemara-mcp:<VERSION>",
               "serve"]
    },
    "complypack": {
      "command": "docker",
      "args": ["run", "--rm", "-i",
               "ghcr.io/complytime/complypack:<VERSION>",
               "mcp", "serve",
               "--source", "<SOURCE>",
               "--schema", "<SCHEMA>"]
    }
  }
}
```

### Step 7: Verify

Check that each server starts and responds. Report loaded catalogs and schemas.

**Claude Code**: Inform user to use `/comply:pipeline` or `/comply:pack`.

**OpenCode**: Inform user to use `/comply-pipeline` or `/comply-pack` (custom commands) or to ask "run the comply pipeline" (skill-based invocation).
