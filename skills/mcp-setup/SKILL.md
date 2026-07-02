---
name: mcp-setup
description: Configure the Gemara and complypack MCP servers for this project — set up artifact sources, platform schemas, and generate the .mcp.json config
---

# /comply:mcp-setup — Configure MCP Servers

Set up the Gemara MCP server and the complypack MCP server for this project.

## MCP Servers

| Server         | Purpose                              | Provides                                                                                                                                                                                                                   |
| -------------- | ------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **gemara**     | Authoring and validation             | `gemara://lexicon`, `gemara://schema/definitions`, `validate_gemara_artifact`                                                                                                                                              |
| **complypack** | Artifact serving, policy validation  | `complypack://catalog/*`, `complypack://mapping/*`, `complypack://schema/*`, `validate_policy`, `test_policy`, `get_assessment_requirements`, `get_applicability_groups`, `get_automation_triage`, `analyze_parameter_delta` |

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

- `ci-github-actions`, `ci-gitlab`, `ci-azure-pipelines` (built-in)
- `kubernetes-deployment`, `kubernetes-pod`, etc. (built-in, per resource type)
- `terraform=https://example.com/terraform.json` (custom, explicit source)

### Step 4: Resolve Versions

Look up latest release versions. Do NOT use `:latest` tags.

- **gemara-mcp**: `gh api repos/gemaraproj/gemara-mcp/releases/latest --jq '.tag_name'`
- **complypack**: `gh api repos/complytime/complypack/releases --jq '.[0].tag_name'`

The complypack releases may be pre-releases, so use the first entry from the
full releases list rather than the `releases/latest` endpoint.

If no release exists, fall back to `:main`.

#### Verify container image tag exists

After resolving the version tag, verify the container image actually exists
at that tag before using it:

```bash
podman manifest inspect ghcr.io/complytime/complypack:<VERSION> > /dev/null 2>&1
```

If the manifest check fails, the release tag does not have a corresponding
container image. Fall back to `:main` and inform the user:

> "No container image found for tag `<VERSION>`. Using `:main` instead.
> The `:main` tag tracks the latest commit on the main branch."

### Step 5: Detect Tool Environment

Determine which AI coding tool is running and adapt the output.

First, scan for all recognized tool directories:

- `.claude-plugin/` → Claude Code
- `.opencode/` → OpenCode
- `.cursor-plugin/` → Cursor

**If multiple tool directories are found**: prompt the user to select their active tool before proceeding. Example:

> Multiple AI coding tools detected in this repository:
> 1. Claude Code (`.claude-plugin/`)
> 2. OpenCode (`.opencode/`)
>
> Which tool are you using? (This only affects post-setup guidance —
> the `.mcp.json` output is identical for all tools.)

**If exactly one is found**: use it automatically.

**If none are found**: fall back to Unknown.

Then apply the selected tool's setup steps:

- **Claude Code**: Write `.mcp.json`.
- **OpenCode**: Write `.mcp.json`. Verify that `.opencode/skills/` symlinks exist — if not, create them:
  ```bash
  mkdir -p .opencode/skills
  ln -sf ../../skills/audit-pipeline .opencode/skills/audit-pipeline
  ln -sf ../../skills/pack-assessment .opencode/skills/pack-assessment
  ln -sf ../../skills/mcp-setup .opencode/skills/mcp-setup
  ```
- **Cursor**: Write `.mcp.json`.
- **Unknown**: Write `.mcp.json` and inform the user about skill discovery.

### Step 6: Write Configuration

Write `.mcp.json`:

```json
{
  "mcpServers": {
    "gemara": {
      "command": "podman",
      "args": ["run", "--rm", "-i",
               "ghcr.io/gemaraproj/gemara-mcp:<VERSION>",
               "serve"]
    },
    "complypack": {
      "command": "podman",
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

**Claude Code**: Inform user to use `/comply:audit-pipeline` or `/comply:pack-assessment`.

**OpenCode**: Inform user to use `/comply-audit-pipeline` or `/comply-pack-assessment` (custom commands) or to ask "run the comply pipeline" (skill-based invocation).
