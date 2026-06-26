# Installing ComplyPack

ComplyPack is a plugin that provides a compliance policy generation skill and
an MCP server for working with Gemara catalogs.

## Prerequisites

- Docker or Podman (Fedora users: `sudo dnf install podman-docker`)

## Claude Code

Install from the marketplace:

```
/plugin install complypack@claude-plugins-official
```

The skill is auto-discovered. To configure the MCP server, create a
`.mcp.json` in your project:

```json
{
  "mcpServers": {
    "complypack": {
      "command": "docker",
      "args": ["run", "--rm", "-i",
               "ghcr.io/complytime/complypack:latest",
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
         "ghcr.io/complytime/complypack:latest",
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

## OpenCode

Skills and custom commands are auto-discovered from `.opencode/skills/`
and `.opencode/commands/` (committed as symlinks). No manual setup needed.

To configure the MCP server, create a `.mcp.json` in your project:

```json
{
  "mcpServers": {
    "complypack": {
      "command": "docker",
      "args": ["run", "--rm", "-i",
               "ghcr.io/complytime/complypack:latest",
               "mcp", "serve",
               "--source", "oci://your-registry/gemara/your-catalog:v1",
               "--schema", "ci"]
    }
  }
}
```

Or use the setup command to generate it interactively:

```
/comply-setup
```

### Available commands

| Command | Description |
|---------|-------------|
| `/comply-pipeline` | Run the scoping, mapping, adherence pipeline |
| `/comply-pack` | Generate Rego policies from the child policy |
| `/comply-setup` | Configure the MCP server for this project |

## Using a config file (advanced)

If you prefer YAML configuration, mount a `complypack.yaml`:

```json
"args": ["run", "--rm", "-i",
         "-v", "./complypack.yaml:/config/complypack.yaml:ro",
         "ghcr.io/complytime/complypack:latest",
         "mcp", "serve",
         "--config", "/config/complypack.yaml"]
```

## Verifying the image

Images include SLSA provenance and SBOM attestations. To verify:

```
gh attestation verify oci://ghcr.io/complytime/complypack:latest \
  --owner complytime
```

## Embedded schemas

These platforms have built-in schemas (no `--schema source` needed):

- `kubernetes`
- `terraform`
- `docker`
- `ansible`
- `ci`
