---
name: setup
description: Set up complypack for this project — generate complypack.yaml, warm the artifact cache, and optionally configure MCP integration
---

# /comply:setup — Set Up ComplyPack

Set up complypack for this project. The primary outputs are a validated
`complypack.yaml` and a warm artifact cache. MCP server configuration is
an optional final step.

## Process

### Step 1: Check Existing Configuration

Check if `complypack.yaml` already exists in the current directory.

- **If it exists**: show its contents and ask the user:
  > `complypack.yaml` already exists. What would you like to do?
  > 1. Reconfigure (regenerate from scratch)
  > 2. Skip to cache warmup (keep current config)
  > 3. Abort

  If the user selects "Reconfigure", continue to Step 2.
  If the user selects "Skip to cache warmup", jump to Step 3.
  If the user selects "Abort", stop.

- **If it does not exist**: continue to Step 2.

### Step 2: Generate `complypack.yaml`

Check if the `complypack` binary is available:

```bash
command -v complypack &> /dev/null && HAVE_BINARY=true || HAVE_BINARY=false
```

#### 2a: Binary available — delegate to `complypack init`

Run `complypack init` in the project directory. The init command handles
all configuration interactively:

- Pack ID (reverse-domain notation, e.g. `io.complytime.my-pack`)
- Version (semver, e.g. `0.1.0`)
- Evaluator plugin (currently only `opa`)
- Platform schemas (multi-select from the built-in schema index)
- Gemara source URI (OCI or file://)

The init command validates the generated config against the JSON Schema
and writes `complypack.yaml` to the current directory.

For non-interactive environments, use flags:

```bash
complypack init \
  --schema kubernetes-deployment \
  --schema ci-github-actions \
  --source oci://ghcr.io/org/catalog:v1 \
  --evaluator-id opa \
  --id io.complytime.my-pack \
  --version 0.1.0
```

If the file already exists, `complypack init` prompts for confirmation
before overwriting (or use `--force` to skip the prompt).

#### 2b: Binary not available

Inform the user that the `complypack` CLI is required:

> The `complypack` binary was not found on PATH. Install it first:
>
> ```bash
> go install github.com/complytime/complypack/cmd/complypack@latest
> ```
>
> Then run `/comply-setup` again.

Stop and wait for the user to install the binary.

### Step 3: Warm the Cache

After `complypack.yaml` exists, pre-warm the artifact cache by running:

```bash
complypack pull
```

This reads the configured Gemara sources and pulls each OCI source into
the persistent cache at `$XDG_CACHE_HOME/complypack` (or
`$HOME/.cache/complypack`). File sources (`file://`) are skipped since
they are already local.

Report the result:

- **All sources pulled**: "Cache warmed: N source(s) pulled."
- **Some sources failed**: Show the errors and ask if the user wants to
  continue. Registry authentication issues are the most common cause —
  suggest `podman login` or `docker login` if the error mentions
  authentication.
- **All sources are file://**: "No OCI sources to cache. All sources are
  local file references."

### Step 4: Verify Configuration

Confirm the setup is ready:

1. `complypack.yaml` exists and passed validation (already ensured by
   `complypack init`)
2. Sources resolved (pull succeeded or all sources are local)
3. Report: number of sources configured, schemas selected, evaluator ID

Example output:

> Setup complete:
> - Config: `complypack.yaml`
> - Sources: 2 OCI, 1 local
> - Schemas: `kubernetes-deployment`, `ci-github-actions`
> - Evaluator: `opa`
> - Cache: warm

### Step 5: Configure MCP (Optional)

Ask if the user wants MCP integration:

> Would you like to configure an MCP server for your AI coding tool?
> 1. Yes — local binary
> 2. Yes — container (Docker/Podman)
> 3. No — skip MCP configuration

If the user selects "No", inform them they can use the CLI directly
(`complypack pack`, `complypack pull`) and stop.

#### 5a: Detect Tool Environment

Determine which AI coding tool is running and adapt the output format.

**Implicit context (preferred):** The agent inherently knows which tool
it is running inside — use it directly without environment checks.

- **OpenCode** — If this skill was loaded via OpenCode's `skill` tool or
  a `/comply-setup` custom command, the tool is OpenCode.
- **Claude Code** — If the skill was loaded via Claude Code's slash
  command system (e.g., `/comply:setup`), the tool is Claude Code.
- **Cursor** — If the skill was loaded via Cursor's command system, the
  tool is Cursor.

**Directory scanning (fallback):** If the agent cannot determine its
runtime identity, scan for tool directories:

- `.claude-plugin/` → Claude Code
- `.opencode/` → OpenCode
- `.cursor-plugin/` → Cursor

If multiple are found, prompt the user to select their active tool.

**Interactive prompt (last resort):** If no tool is detected, prompt:

> Which AI coding tool are you using?
> 1. Claude Code
> 2. OpenCode
> 3. Cursor

#### 5b: Local Binary MCP Configuration

The complypack binary reads `complypack.yaml` from the current directory
by default. The MCP server command is:

```bash
complypack mcp serve
```

Or with an explicit config path:

```bash
complypack mcp serve --config /path/to/complypack.yaml
```

Write the tool-specific config file:

**Claude Code / Cursor — `.mcp.json`:**

```json
{
  "mcpServers": {
    "complypack": {
      "command": "complypack",
      "args": ["mcp", "serve"]
    }
  }
}
```

**OpenCode — `opencode.json`:**

If `opencode.json` already exists, merge the `mcp` entry into it.
If not, create a new file.

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "complypack": {
      "type": "local",
      "command": ["complypack", "mcp", "serve"]
    }
  }
}
```

> **Key differences:** OpenCode uses `opencode.json` with top-level key
> `mcp` (not `mcpServers`). Each server has `"type": "local"` and
> `command` is a single array (not split into `command` + `args`).

#### 5c: Container MCP Configuration

Detect which container runtime is available:

```bash
command -v docker &> /dev/null && HAVE_DOCKER=true || HAVE_DOCKER=false
command -v podman &> /dev/null && HAVE_PODMAN=true || HAVE_PODMAN=false
```

- If **only one** is found: use it and set `RUNTIME`.
- If **both** are found: ask the user which they prefer.
- If **neither** is found: inform the user that a container runtime is
  required and offer to fall back to local binary (Step 5b).

**Resolve container image version:**

Look up the latest release. Do NOT use `:latest` tags.

```bash
gh api repos/complytime/complypack/releases --jq '.[0].tag_name'
```

If no release exists, fall back to `:main`.

Verify the container image tag exists:

```bash
<RUNTIME> manifest inspect ghcr.io/complytime/complypack:<VERSION> > /dev/null 2>&1
```

If the manifest check fails, fall back to `:main` and inform the user:

> No container image found for tag `<VERSION>`. Using `:main` instead.

**Write configuration:**

The container needs the `complypack.yaml` mounted in and the source/schema
values passed via `--source` and `--schema` flags (read from the generated
`complypack.yaml`).

**Claude Code / Cursor — `.mcp.json`:**

```json
{
  "mcpServers": {
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

**OpenCode — `opencode.json`:**

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
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

> **Volume mounts for `file://` sources:** When any source uses `file://`,
> the container command must include `-v <host-path>:/workspace -w /workspace`
> to mount the host directory. Without this, the server cannot access the
> file. On SELinux systems (Fedora, RHEL), add `:z` to the volume mount.

### Step 6: Verify MCP (if configured)

Check that the MCP server starts and responds. Report loaded catalogs
and schemas.

**Claude Code**: Inform user to use `/comply:audit-pipeline` or
`/comply:build-assessment`.

**OpenCode**: Inform user to use `/comply-pipeline` or `/comply-pack`
(custom commands) or to ask "run the comply pipeline" (skill-based
invocation).

## MCP Server

| Server         | Purpose                              | Provides                                                                                                                                                                                                                   |
| -------------- | ------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **complypack** | Artifact serving, policy validation  | `complypack://catalog/*`, `complypack://mapping/*`, `complypack://schema/*`, `validate_policy`, `test_policy`, `get_assessment_requirements`, `get_applicability_groups`, `get_automation_triage`, `analyze_parameter_delta` |
