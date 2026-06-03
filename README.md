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
# The provider plugin that evaluates this pack's content (must match the
# provider's registered ID — e.g., "opa" matches the opa-provider plugin).
# Used by complyctl to dispatch content to the correct provider, by the
# MCP server to select the right validator/tester, and embedded in the
# OCI artifact config layer so consumers know how to interpret the content.
evaluator-id: opa

# Version of this complypack artifact
version: 0.1.0

# Gemara policy source (for MCP server)
gemara:
  source: oci://ghcr.io/org/controls:v1

# Platform schemas (for MCP server validation tools)
schemas:
  - platform: kubernetes
    source: cue://cue.dev/x/k8s.io/api/core/v1
  - platform: ci
    source: cue://cue.dev/x/gitlab/gitlabci
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

| Resource | Description |
|----------|-------------|
| `complypack://catalog/<name>` | Gemara catalog (YAML) |
| `complypack://schema/<platform>` | Platform schema (JSON) |
| `complypack://evaluator` | Available policy evaluators |

#### MCP Tools

| Tool | Description |
|------|-------------|
| `validate_policy` | Validate policy syntax, contract compliance, and linting |
| `test_policy` | Run policy against test data with schema validation |
| `get_assessment_requirements` | Extract assessment requirements with parameters |

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
