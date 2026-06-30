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
         "ghcr.io/complytime/complypack:latest",
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
               "--schema", "ci-github-actions"]
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
