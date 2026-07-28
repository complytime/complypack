# Example Gemara Artifacts

Minimal catalog, policy, and guidance for testing the comply pipeline locally.

## Files

```
examples/gemara/
  control-catalog.yaml   # 2 controls, 4 assessment requirements
  policy.yaml            # Imports catalog + guidance, sets parameters
  guidance-catalog.yaml  # 2 guidelines with recommendations
```

## Testing with the MCP server

Build and run:

```bash
go build -o complypack ./cmd/complypack
./complypack mcp serve \
  --source file://./examples/gemara/control-catalog.yaml \
  --source file://./examples/gemara/policy.yaml \
  --source file://./examples/gemara/guidance-catalog.yaml \
  --schema kubernetes-deployment
```

## Testing with Claude Code

Add to `.claude/settings.json` under `mcpServers`:

```json
{
  "complypack": {
    "command": "./complypack",
    "args": [
      "mcp", "serve",
      "--source", "file://./examples/gemara/control-catalog.yaml",
      "--source", "file://./examples/gemara/policy.yaml",
      "--source", "file://./examples/gemara/guidance-catalog.yaml",
      "--schema", "kubernetes-deployment"
    ]
  }
}
```

Then invoke the pipeline skill: `/comply:audit-pipeline`

## Testing with OpenCode

OpenCode discovers skills from `.opencode/skills/`, not from plugin manifests.
Create symlinks (`.opencode/` is already gitignored):

```bash
mkdir -p .opencode/skills
ln -s ../../skills/audit-pipeline .opencode/skills/audit-pipeline
ln -s ../../skills/build-assessment .opencode/skills/build-assessment
ln -s ../../skills/setup .opencode/skills/setup
```

Add MCP server config to `.mcp.json`:

```json
{
  "mcpServers": {
    "complypack": {
      "command": "./complypack",
      "args": [
        "mcp", "serve",
        "--source", "file://./examples/gemara/control-catalog.yaml",
        "--source", "file://./examples/gemara/policy.yaml",
        "--source", "file://./examples/gemara/guidance-catalog.yaml"
      ]
    }
  }
}
```
