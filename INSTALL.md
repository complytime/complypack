# Installing ComplyPack

ComplyPack is a plugin that provides a compliance policy generation skill and
an MCP server for working with Gemara catalogs.

## Prerequisites

- Podman or Docker

> **Note:** All examples below use `podman`. Docker users can substitute
> `docker` directly — the commands are interchangeable. The setup command
> (`/comply:setup` or `/comply-setup`) auto-detects which runtime is
> available and generates the correct configuration.

## Claude Code

Add the ComplyTime marketplace and install the plugin:

```text
/plugin marketplace add complytime/complypack
/plugin install comply@complytime
```

The skills (`/comply:setup`, `/comply:build-assessment`, `/comply:audit-pipeline`) are
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

### Local file sources (development)

When using `file://` sources during local development (before your catalog
is published as an OCI bundle), you must mount the host directory into the
container. Without the volume mount, the server will fail because the file
path does not exist inside the container.

For Claude Code / Cursor (`.mcp.json`):

```json
"args": ["run", "--rm", "-i",
         "-v", "/path/to/artifacts:/workspace",
         "-w", "/workspace",
         "ghcr.io/complytime/complypack:main",
         "mcp", "serve",
         "--source", "file://catalog.yaml",
         "--schema", "ci-github-actions"]
```

For OpenCode (`opencode.json`):

```json
"command": ["podman", "run", "--rm", "-i",
            "-v", "/path/to/artifacts:/workspace",
            "-w", "/workspace",
            "ghcr.io/complytime/complypack:main",
            "mcp", "serve",
            "--source", "file://catalog.yaml",
            "--schema", "ci-github-actions"]
```

The `-v` flag mounts the host directory containing your artifacts, and
`-w /workspace` sets the container's working directory so `file://`
relative paths resolve correctly.

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

To configure the MCP server, add a `complypack` entry to `opencode.json`
(or `opencode.jsonc`) in your project root:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "complypack": {
      "type": "local",
      "command": ["podman", "run", "--rm", "-i",
                  "ghcr.io/complytime/complypack:main",
                  "mcp", "serve",
                  "--source", "oci://your-registry/gemara/your-catalog:v1",
                  "--schema", "ci-github-actions"]
    }
  }
}
```

> **Note:** OpenCode uses `opencode.json`, not `.mcp.json`. The format
> differs: the top-level key is `mcp` (not `mcpServers`), each server
> includes `"type": "local"`, and `command` is a single array (not split
> into `command` + `args`).

Or use the setup command to generate it interactively:

```text
/comply-setup
```

### Available commands

| Command            | Description                                          |
|--------------------|------------------------------------------------------|
| `/comply-setup`    | Configure complypack MCP server for this project     |
| `/comply-pack`     | Generate Rego policies from the child policy         |
| `/comply-pipeline` | Run the comply pipeline (scoping, mapping, adherence)|

## Lola (any AI assistant)

Install the complypack module with [lola](https://lobstertrap.org/lola/):

```bash
lola mod add https://github.com/complytime/complypack.git
lola install complypack -a opencode --scope project
```

This installs skills and commands into `.opencode/skills/` and
`.opencode/commands/`. After installation, the `/comply-setup`,
`/comply-pipeline`, and `/comply-pack` commands are available in OpenCode.

To update after complypack releases new skills:

```bash
lola mod update complypack
lola install complypack -a opencode --scope project
```

> **Note:** The MCP server still requires separate configuration. Run
> `/comply-setup` after installation to generate the MCP config, or
> edit `mcps.json` manually with your registry and schema values.

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

## Troubleshooting

### MCP client shows "Connection closed" with no details

When `complypack mcp serve` fails during startup, the process exits before
the MCP handshake completes. Older versions wrote the error only to stderr,
which most MCP clients discard. Since v0.0.7 the server also writes a
JSON-RPC error to stdout so clients can surface the real message.

If your client still shows a generic error, check stderr manually:

```bash
podman run --rm -i ghcr.io/complytime/complypack:main \
  mcp serve \
  --source "oci://your-registry/gemara/your-catalog:v1" \
  --schema ci-github-actions \
  2>/tmp/complypack-stderr.log; cat /tmp/complypack-stderr.log
```

### Source file or catalog not found

```
Error: failed to create MCP server: failed to load artifacts from
file://./path/to/catalog.yaml: no such file or directory
```

**Causes:**
- `file://` sources reference paths **inside the container**, not on the host.
  Mount the file first with `-v`:
  ```json
  "args": ["run", "--rm", "-i",
           "-v", "./governance:/governance:ro",
           "ghcr.io/complytime/complypack:main",
           "mcp", "serve",
           "--source", "file:///governance/catalog.yaml"]
  ```
- OCI sources require network access. Verify the registry is reachable:
  ```bash
  podman pull oci://your-registry/gemara/your-catalog:v1
  ```

### Permission denied on volume mounts

```
Error: failed to read file: open /config/complypack.yaml: permission denied
```

On SELinux systems (Fedora, RHEL), add the `:z` suffix to volume mounts:

```bash
-v "./complypack.yaml:/config/complypack.yaml:ro,z"
```

See the [SELinux section](#selinux-fedora--rhel) above.

### OCI registry authentication failure

```
Error: failed to pull catalog: authentication failed
```

Log in to the registry before starting the server:

```bash
podman login ghcr.io
# or
docker login your-registry.example.com
```

For GitHub Container Registry, use a personal access token with `read:packages`
scope.

### Unknown or unsupported schema

```
Error: failed to load schemas: schema "foobar" not found in index
```

Check the [Built-in schemas](#built-in-schemas) list above. Schema names are
case-sensitive. For custom platforms, provide the schema source explicitly:

```bash
--schema my-platform=/path/to/schema.cue
```

### Config file errors

```
Error: failed to load config: complypack.yaml: unmarshal error
```

Validate your `complypack.yaml` syntax. Required fields depend on your setup —
at minimum you need sources and at least one schema. See
[Using a config file](#using-a-config-file-advanced).
