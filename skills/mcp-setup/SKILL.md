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

Check if a config file already exists. The file depends on the tool environment:
- Claude Code / Cursor: `.mcp.json`
- OpenCode: `opencode.json` (or `opencode.jsonc`)

Show current config and ask if the user wants to reconfigure.

### Step 2: Detect Container Runtime

Detect which container runtime is available on the host before generating
any configuration.

```bash
command -v docker &> /dev/null && HAVE_DOCKER=true || HAVE_DOCKER=false
command -v podman &> /dev/null && HAVE_PODMAN=true || HAVE_PODMAN=false
```

- If **only one** is found: use it automatically and set `RUNTIME` to
  `docker` or `podman`.
- If **both** are found: ask the user which they prefer and set `RUNTIME`
  to their choice.
- If **neither** is found: stop and inform the user that a container
  runtime (`docker` or `podman`) is required. Do not proceed.

Use `<RUNTIME>` in all subsequent steps wherever the container command
appears.

### Step 3: Configure Sources

Ask for Gemara artifact sources for the complypack server:

- `oci://registry.example.com/gemara/controls:v1`
- `oci+http://localhost:5001/gemara/controls:v1` (development)
- `file://path/to/catalog.yaml`

At least one source is required.

> **Volume mounts for `file://` sources:** When any source uses `file://`,
> the Docker/Podman command must include `-v <host-path>:/workspace -w /workspace`
> to mount the host directory into the container. Without this, the server
> cannot access the file and will fail at startup. Relative `file://` paths
> resolve from the container's working directory (`/workspace`).

### Step 4: Configure Schemas

Ask which platform schemas to load:

- `ci-github-actions`, `ci-gitlab`, `ci-azure-pipelines` (built-in)
- `kubernetes-deployment`, `kubernetes-pod`, etc. (built-in, per resource type)
- `terraform=https://example.com/terraform.json` (custom, explicit source)

### Step 5: Resolve Versions

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
<RUNTIME> manifest inspect ghcr.io/complytime/complypack:<VERSION> > /dev/null 2>&1
```

If the manifest check fails, the release tag does not have a corresponding
container image. Fall back to `:main` and inform the user:

> "No container image found for tag `<VERSION>`. Using `:main` instead.
> The `:main` tag tracks the latest commit on the main branch."

### Step 6: Detect Tool Environment

Determine which AI coding tool is running and adapt the output.

#### 5a: Implicit context (preferred)

The most reliable signal is the tool that is currently executing this skill.
Users may have multiple AI tools installed, so binary/PATH detection is not
sufficient — it only proves installation, not active use.

- **OpenCode** — If this skill was loaded via OpenCode's `skill` tool or
  a `/comply-setup` custom command, the tool is OpenCode. The agent
  should recognize that it *is* OpenCode and select itself without
  scanning.
- **Claude Code** — If the skill was loaded via Claude Code's slash
  command system (e.g., `/comply:mcp-setup`), the tool is Claude Code.
- **Cursor** — If the skill was loaded via Cursor's command system, the
  tool is Cursor.

Since the agent inherently knows which tool it is running inside, this
detection requires no environment checks. Use it directly and skip all
further detection steps.

#### 5b: Directory scanning (fallback)

If the agent cannot determine its own runtime identity (e.g., the skill
instructions were copy-pasted into a generic chat), scan for recognized
tool directories:

- `.claude-plugin/` → Claude Code
- `.opencode/` → OpenCode
- `.cursor-plugin/` → Cursor

**If multiple tool directories are found**: prompt the user to select their
active tool before proceeding. Example:

> Multiple AI coding tools detected in this repository:
> 1. Claude Code (`.claude-plugin/`)
> 2. OpenCode (`.opencode/`)
>
> Which tool are you using? (This affects the config file format and
> post-setup guidance.)

**If exactly one is found**: use it automatically.

#### 5c: Interactive prompt (last resort)

**If the agent cannot determine its runtime identity and no tool
directories are found**, prompt the user — never silently default:

> No AI coding tool detected automatically.
> Which AI coding tool are you using?
>
> 1. Claude Code
> 2. OpenCode
> 3. Cursor

#### Fallback rule

If any step above fails or produces an ambiguous result, always fall back
to prompting the user with the supported tool list. Never guess silently.

#### Apply the detected tool's setup steps

- **Claude Code**: Write `.mcp.json` (see Step 7, `.mcp.json` format).
- **OpenCode**: Write `opencode.json` (see Step 7, `opencode.json` format). Verify that `.opencode/skills/` symlinks exist — if not, create them:
  ```bash
  mkdir -p .opencode/skills
  ln -sf ../../skills/audit-pipeline .opencode/skills/audit-pipeline
  ln -sf ../../skills/pack-assessment .opencode/skills/pack-assessment
  ln -sf ../../skills/mcp-setup .opencode/skills/mcp-setup
  ```
- **Cursor**: Write `.mcp.json` (see Step 7, `.mcp.json` format).

### Step 7: Write Configuration

#### Claude Code / Cursor — `.mcp.json`

```json
{
  "mcpServers": {
    "gemara": {
      "command": "<RUNTIME>",
      "args": ["run", "--rm", "-i",
               "ghcr.io/gemaraproj/gemara-mcp:<VERSION>",
               "serve"]
    },
    "complypack": {
      "command": "<RUNTIME>",
      "args": ["run", "--rm", "-i",
               "ghcr.io/complytime/complypack:<VERSION>",
               "mcp", "serve",
               "--source", "<SOURCE>",
               "--schema", "<SCHEMA>"]
    }
  }
}
```

#### OpenCode — `opencode.json`

If `opencode.json` already exists, merge the `mcp` entries into it.
If not, create a new file.

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "gemara": {
      "type": "local",
      "command": ["<RUNTIME>", "run", "--rm", "-i",
                  "ghcr.io/gemaraproj/gemara-mcp:<VERSION>",
                  "serve"]
    },
    "complypack": {
      "type": "local",
      "command": ["<RUNTIME>", "run", "--rm", "-i",
                  "ghcr.io/complytime/complypack:<VERSION>",
                  "mcp", "serve",
                  "--source", "<SOURCE>",
                  "--schema", "<SCHEMA>"]
    }
  }
}
```

> **Key differences:** OpenCode uses `opencode.json` with top-level key
> `mcp` (not `mcpServers`). Each server has `"type": "local"` and
> `command` is a single array (not split into `command` + `args`).

### Step 8: Verify

Check that each server starts and responds. Report loaded catalogs and schemas.

**Claude Code**: Inform user to use `/comply:audit-pipeline` or `/comply:pack-assessment`.

**OpenCode**: Inform user to use `/comply-pipeline` or `/comply-pack` (custom commands) or to ask "run the comply pipeline" (skill-based invocation).
