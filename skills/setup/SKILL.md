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
- **complypack**: `gh api repos/complytime/complypack/releases --jq '.[0].tag_name'`

The complypack releases may be pre-releases, so use the first entry from the
full releases list rather than the `releases/latest` endpoint.

If no release exists, ask the user for a version to pin.

### Step 5: Write Configuration

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

### Step 6: Verify

Check that each server starts and responds. Report loaded catalogs and schemas.

> "MCP servers configured. Use `/comply:pipeline` to start the compliance pipeline or `/comply:pack` to generate assessment logic."
