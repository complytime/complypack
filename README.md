# ComplyPack

[![CI](https://github.com/complytime/complypack/actions/workflows/ci.yml/badge.svg)](https://github.com/complytime/complypack/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/complytime/complypack.svg)](https://pkg.go.dev/github.com/complytime/complypack)
[![Go Report Card](https://goreportcard.com/badge/github.com/complytime/complypack)](https://goreportcard.com/report/github.com/complytime/complypack)

ComplyPack is a CLI and Go library for packing, unpacking, signing, and verifying OCI artifacts containing policy bundles. It provides an evaluator-agnostic format for distributing compliance policies using OCI registries, and an MCP server for LLM-assisted policy generation.

## Features

- **OCI Artifact Packaging** - Pack policy content into OCI Image Manifest v1.1 artifacts
- **MCP Server** - Expose Gemara catalogs, platform schemas, and evaluators to LLMs
- **Policy Graph Resolution** - Resolve effective policies with overlays from Gemara bundles
- **Evaluator-Agnostic** - Supports any policy language (OPA, CEL, etc.) via evaluator-id dispatch
- **CUE Schema Sources** - Load platform schemas from CUE registry, HTTPS, or local files
- **Signing & Verification** - Built-in support for keyed and keyless (Sigstore) signing

## Installation

### CLI

```bash
go install github.com/complytime/complypack/cmd/complypack@latest
```

### Library

```bash
go get github.com/complytime/complypack
```

## Configuration

Create `complypack.yaml` in your working directory:

```yaml
# Globally unique pack identifier (reverse-domain convention).
# Survives registry moves, distinguishes packs from different authors.
id: io.complytime.my-controls

# Provider plugin that evaluates this pack's content.
# Must match the provider's binary suffix (e.g., "opa" → complyctl-provider-opa).
evaluator-id: opa

# ComplyPack artifact version
version: 0.1.0

# Gemara policy source (for MCP server)
gemara:
  source: oci://ghcr.io/org/controls:v1

# Platform schemas (for MCP server validation tools)
# Built-in platforms: ci-github-actions, ci-gitlab, ci-azure-pipelines,
# kubernetes-deployment, kubernetes-pod, etc. (see schemas/index.yaml)
schemas:
  - platform: kubernetes-deployment
  - platform: ci-github-actions
```

See `complypack.example.yaml` for full configuration options.

### Authentication

Uses the Docker credential chain:

```bash
docker login ghcr.io
```

## CLI Usage

### Pack

Pack a directory of policy content into a ComplyPack OCI artifact and push to a registry:

```bash
# Pack and push to a registry
complypack pack policy/ ghcr.io/org/my-policies:v1.0.0

# Pack to a local registry
complypack pack policy/ localhost:5001/test:latest --plain-http
```

The command reads `evaluator-id` and `version` from `complypack.yaml`. The content directory is tar+gzipped and stored as the artifact's opaque content layer.

### MCP Server

Start the MCP server to expose Gemara catalogs, platform schemas, and policy tools to LLMs:

```bash
complypack mcp serve
complypack mcp serve --config /path/to/complypack.yaml
```

#### MCP Resources

| Resource                         | Description                |
|----------------------------------|----------------------------|
| `complypack://catalog/<name>`    | Gemara catalog (YAML)      |
| `complypack://schema/<platform>` | Platform schema (JSON)     |
| `complypack://evaluator`         | Available policy evaluators |

#### MCP Tools

| Tool                           | Description                                               |
|--------------------------------|-----------------------------------------------------------|
| `validate_policy`              | Validate policy syntax, contract compliance, and linting  |
| `test_policy`                  | Run policy against test data with schema validation       |
| `get_assessment_requirements`  | Extract assessment requirements with parameters           |
| `get_applicability_groups`     | Get group definitions and requirement memberships         |
| `get_automation_triage`        | Classify assessment plans as Automated or Manual          |
| `analyze_parameter_delta`      | Compare L3 parameter values against L1/L2 requirements    |

#### Tested AI Coding Tools

The MCP server and skills have been tested with:

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code)
- [OpenCode](https://opencode.ai)

### AI Tool Setup

ComplyPack is available as a plugin for Claude Code, Gemini CLI, and OpenCode.
Cursor is also supported via MCP server configuration.
See [INSTALL.md](INSTALL.md) for setup instructions.

## Architecture

### ComplyPack OCI Artifact

```json
{
  "artifactType": "application/vnd.complypack.artifact.v1",
  "config": { "mediaType": "application/vnd.complypack.config.v1+json" },
  "layers": [{ "mediaType": "application/vnd.complypack.content.v1.tar+gzip" }]
}
```

| Purpose       | Media Type                                       |
|---------------|--------------------------------------------------|
| Artifact Type | `application/vnd.complypack.artifact.v1`         |
| Config Layer  | `application/vnd.complypack.config.v1+json`      |
| Content Layer | `application/vnd.complypack.content.v1.tar+gzip` |

The content layer is **opaque** — the `evaluator-id` in the config tells consumers which provider handles it. For OPA, this is a tarball of `.rego` files.

### Policy Graph Resolution

The MCP server resolves Gemara policy graphs:

1. Load OCI bundle or local file
2. `bundle.Classify()` — identify artifact types (Policy, ControlCatalog, etc.)
3. `ResolveEffectivePolicy()` — apply overlays from policy imports
4. Extract assessment requirements with structured parameters from assessment plans

## Library Quick Start

### Packing

```go
cfg := complypack.Config{
    ID:          "io.example.my-policies",
    EvaluatorID: "opa",
    Version:     "1.0.0",
}

content := strings.NewReader("policy content here")
desc, err := complypack.Pack(ctx, store, cfg, content)
```

### Unpacking

```go
result, err := complypack.Unpack(ctx, store, desc)
defer result.Content.Close()

fmt.Printf("Evaluator: %s\n", result.Config.EvaluatorID)
```

## Error Handling

ComplyPack uses sentinel errors:

- `ErrInvalidConfig` — Config validation failed
- `ErrEmptyContent` — Content reader returned zero bytes
- `ErrContentTooLarge` — Content exceeds 100MB limit
- `ErrSigningFailed` — Signing operation failed
- `ErrVerificationFailed` — Signature verification failed
- `ErrInvalidMediaType` — Unexpected media type in manifest
- `ErrNoContentLayer` — Manifest missing content layer

## Current Limitations

- **Signing/Verification**: Validation logic is implemented, but full sigstore-go integration is pending
- **Content Size**: Maximum 100MB per artifact
- **Single Content Layer**: Only one content layer per artifact is supported

## Related Projects

- [ComplyTime](https://github.com/complytime) — Compliance automation
- [Gemara](https://github.com/gemaraproj/gemara) — Compliance policy framework
- [ORAS](https://oras.land/) — OCI Registry as Storage
- [Open Policy Agent](https://www.openpolicyagent.org/) — Policy-based control

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
