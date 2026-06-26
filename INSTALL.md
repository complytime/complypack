# Installing ComplyPack

ComplyPack is a plugin that provides a compliance policy generation skill and
an MCP server for working with Gemara catalogs.

## Prerequisites

- Docker or Podman (Fedora users: `sudo dnf install podman-docker`)

## Claude Code

Add the ComplyTime marketplace and install the plugin:

```
/plugin marketplace add complytime/complypack
/plugin install comply@complytime
```

The skills (`/comply:setup`, `/comply:pack`, `/comply:pipeline`) are
auto-discovered once the plugin is installed. To configure the MCP server,
create a `.mcp.json` in your project:

```json
{
  "mcpServers": {
    "complypack": {
      "command": "docker",
      "args": ["run", "--rm", "-i",
               "ghcr.io/complytime/complypack:main",
               "mcp", "serve",
               "--source", "oci://your-registry/gemara/your-catalog:v1",
               "--schema", "ci"]
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
         "--schema", "ci=cue://cue.dev/x/githubactions@v0#Workflow",
         "--schema", "kubernetes"]
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
      "command": "docker",
      "args": ["run", "--rm", "-i",
               "ghcr.io/complytime/complypack:main",
               "mcp", "serve",
               "--source", "oci://your-registry/gemara/your-catalog:v1",
               "--schema", "ci"]
    }
  }
}
```

The skills in `skills/` are available when the complypack repository is open
in Cursor as part of the workspace.

## OpenCode

Add to your `opencode.json`:

```json
{
  "mcpServers": {
    "complypack": {
      "command": "docker",
      "args": ["run", "--rm", "-i",
               "ghcr.io/complytime/complypack:main",
               "mcp", "serve",
               "--source", "oci://your-registry/gemara/your-catalog:v1",
               "--schema", "ci"]
    }
  }
}
```

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

```
gh attestation verify oci://ghcr.io/complytime/complypack:main \
  --owner complytime
```

## Embedded schemas

These platforms have built-in schemas (no `--schema source` needed):

- `kubernetes`
- `terraform`
- `docker`
- `ansible`
- `ci`
