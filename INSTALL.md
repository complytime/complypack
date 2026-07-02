# Installing ComplyPack

ComplyPack is a plugin that provides a compliance policy generation skill and
an MCP server for working with Gemara catalogs.

## Prerequisites

- Podman or Docker

> **Note:** All examples below use `podman`. Docker users can substitute
> `docker` directly — the commands are interchangeable.

## Claude Code

Add the ComplyTime marketplace and install the plugin:

```text
/plugin marketplace add complytime/complypack
/plugin install comply@complytime
```

The skills (`/comply:mcp-setup`, `/comply:pack-assessment`, `/comply:audit-pipeline`) are
auto-discovered once the plugin is installed. To configure the MCP server,
create a `.mcp.json` in your project:

```json
{
  "mcpServers": {
    "complypack": {
      "command": "podman",
      "args": ["run", "--rm", "-i",
               "ghcr.io/complytime/complypack:main",
               "mcp", "serve",
               "--source", "oci://your-registry/gemara/your-catalog:v1",
               "--schema", "ci-github-actions"]
    }
  }
}
```

Replace the `--source` and `--schema` values with your Gemara catalog
references and target platforms.

### Multiple sources and schemas

```json
"args": ["run", "--rm", "-i",
         "ghcr.io/complytime/complypack:main",
         "mcp", "serve",
         "--source", "oci://registry.example.com/gemara/controls:v1",
         "--source", "oci://registry.example.com/gemara/guidance:v1",
         "--schema", "ci-github-actions",
         "--schema", "kubernetes-deployment"]
```

### Plain HTTP registries (development)

Use `oci+http://` for registries without TLS:

```json
"--source", "oci+http://localhost:5001/gemara/controls:v1"
```

## Cursor

Add the MCP server to your Cursor settings. Open **Settings > MCP** and add
a new server with the following configuration:

```json
{
  "mcpServers": {
    "complypack": {
      "command": "podman",
      "args": ["run", "--rm", "-i",
               "ghcr.io/complytime/complypack:main",
               "mcp", "serve",
               "--source", "oci://your-registry/gemara/your-catalog:v1",
               "--schema", "ci-github-actions"]
    }
  }
}
```

## Gemini CLI

Install the extension:

```bash
gemini extensions install https://github.com/complytime/complypack
```

For local development, link instead of install:

```bash
gemini extensions link /path/to/complypack
```

Verify the extension is loaded:

```bash
gemini extensions list
```

The following slash commands are available in a Gemini session:

| Command      | Description                                  |
|--------------|----------------------------------------------|
| `/setup`     | Configure MCP servers for this project        |
| `/pack`      | Generate Rego policies from Gemara catalogs   |
| `/pipeline`  | Run the scoping, mapping, adherence pipeline  |

## OpenCode

Skills and custom commands are auto-discovered from `.opencode/skills/`
and `.opencode/commands/` (committed as symlinks). No manual setup needed.

To configure the MCP server, create a `.mcp.json` in your project:

```json
{
  "mcpServers": {
    "complypack": {
      "command": "podman",
      "args": ["run", "--rm", "-i",
               "ghcr.io/complytime/complypack:main",
               "mcp", "serve",
               "--source", "oci://your-registry/gemara/your-catalog:v1",
               "--schema", "ci-github-actions"]
    }
  }
}
```

Or use the setup command to generate it interactively:

```text
/comply-setup
```

### Available commands

| Command            | Description                                  |
|--------------------|----------------------------------------------|
| `/comply-pipeline` | Run the scoping, mapping, adherence pipeline |
| `/comply-pack`     | Generate Rego policies from the child policy |
| `/comply-setup`    | Configure the MCP server for this project    |

## SELinux (Fedora / RHEL)

On systems with SELinux enforcing, volume mounts require the `:z` suffix so
the container process can read the files:

```json
"args": ["run", "--rm", "-i",
         "-v", "./complypack.yaml:/config/complypack.yaml:ro,z",
         "ghcr.io/complytime/complypack:main",
         "mcp", "serve",
         "--config", "/config/complypack.yaml"]
```

Without `:z` you will see `permission denied` errors when the server tries
to load sources from mounted paths.

## Using a config file (advanced)

If you prefer YAML configuration, mount a `complypack.yaml`:

```json
"args": ["run", "--rm", "-i",
         "-v", "./complypack.yaml:/config/complypack.yaml:ro,z",
         "ghcr.io/complytime/complypack:main",
         "mcp", "serve",
         "--config", "/config/complypack.yaml"]
```

## Verifying the image

Images include SLSA provenance and SBOM attestations. To verify:

```bash
gh attestation verify oci://ghcr.io/complytime/complypack:main \
  --owner complytime
```

## Built-in schemas

These platforms are in the schema index (no explicit source needed):

**CI/CD:**
- `ci-github-actions`
- `ci-gitlab`
- `ci-azure-pipelines`

**Kubernetes** (per resource type):
- `kubernetes-deployment`, `kubernetes-pod`, `kubernetes-daemonset`,
  `kubernetes-statefulset`, `kubernetes-cronjob`, `kubernetes-job`,
  `kubernetes-service`, `kubernetes-networkpolicy`, `kubernetes-ingress`,
  `kubernetes-role`, `kubernetes-clusterrole`, `kubernetes-rolebinding`,
  `kubernetes-clusterrolebinding`, `kubernetes-serviceaccount`,
  `kubernetes-configmap`, `kubernetes-secret`, `kubernetes-namespace`

Custom platforms (e.g., terraform, docker, ansible) can be registered with
`--schema <name>=<source>` or via `complypack.yaml`.
