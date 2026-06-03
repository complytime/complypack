# ComplyPack

[![CI](https://github.com/complytime/complypack/actions/workflows/ci.yml/badge.svg)](https://github.com/complytime/complypack/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/complytime/complypack.svg)](https://pkg.go.dev/github.com/complytime/complypack)
[![Go Report Card](https://goreportcard.com/badge/github.com/complytime/complypack)](https://goreportcard.com/report/github.com/complytime/complypack)

ComplyPack is a Go library for packing, unpacking, signing, and verifying OCI artifacts containing policy bundles. It provides an evaluator-agnostic format for distributing compliance policies using OCI registries.

## Features

- **OCI Artifact Packaging** - Pack policy content into OCI Image Manifest v1.1 artifacts
- **Evaluator-Agnostic** - Supports any policy language (OPA, CEL, etc.) via evaluator-id dispatch
- **Signing & Verification** - Built-in support for keyed and keyless (Sigstore) signing
- **Memory-Safe** - 100MB content size limit prevents memory exhaustion attacks
- **Provenance Tracking** - Optional Gemara linkage for generated policies
- **Flexible API** - Functional options pattern for clean, extensible configuration

## Installation

### Library

```bash
go get github.com/complytime/complypack
```

### CLI

```bash
go install github.com/complytime/complypack/cmd/complypack@latest
```

## CLI Usage

### Pulling Gemara Catalogs from OCI Registries

The `complypack` CLI can pull Gemara control catalogs from OCI registries:

```bash
# Pull and output to stdout
complypack catalog pull ghcr.io/org/controls:v1.0

# Save to a file
complypack catalog pull ghcr.io/org/controls:v1.0 --output controls.yaml

# Pull from a local registry
complypack catalog pull http://localhost:5000/controls:latest --plain-http
```

#### Authentication

The CLI uses the Docker credential chain (same as `docker login`):

```bash
docker login ghcr.io
complypack catalog pull ghcr.io/org/controls:v1.0
```

Supports:
- Docker credential helpers (`credHelpers`)
- Docker credential store (`credsStore`)
- Docker config file (`auths`)

### MCP Server

Run the Model Context Protocol server to expose Gemara catalogs and platform schemas to LLMs:

```bash
complypack mcp serve
```

#### Prerequisites

1. Create `complypack.yaml` in your working directory:

```yaml
evaluator-id: io.complytime.opa
version: 0.1.0

# Gemara catalog source (file path or OCI reference)
gemara:
  source: ghcr.io/complytime/controls-catalog:v1
  # OR: source: file://./catalogs/my-controls.yaml

# Platform schemas with flexible source types
schemas:
  # CUE registry module (fetches from cue.dev)
  - platform: kubernetes
    source: cue://cue.dev/x/k8s.io/api/core/v1

  # HTTPS download
  - platform: terraform
    source: https://example.com/schemas/terraform.json

  # Local file
  - platform: docker
    source: file://./schemas/docker.cue

  # Embedded fallback (no source specified)
  - platform: ansible
```

See `complypack.example.yaml` for full configuration options.

2. Authenticate to OCI registries if needed:

```bash
docker login ghcr.io
```

#### Resources Exposed

- Catalogs: `complypack://catalog/<name>` (YAML)
- Schemas: `complypack://schema/<platform>` (JSON Schema)

#### Flags

- `--config, -c`: Path to complypack.yaml (default: "./complypack.yaml")
- `--cache-dir`: Cache directory (default: "$HOME/.complypack/cache")

#### Example

```bash
# Start server with default config
complypack mcp serve

# Use custom config path
complypack mcp serve --config /path/to/config.yaml

# Use custom cache directory
complypack mcp serve --cache-dir /tmp/cache

# Connect from Claude Code
# (Server runs on stdio transport - Claude Code MCP client will connect automatically)
```

#### Built-in Platforms

- `kubernetes` - Kubernetes resources (Deployment, Pod, Service, etc.)
- `terraform` - Terraform plan JSON
- `docker` - Dockerfile and container config
- `ansible` - Ansible playbooks
- `ci` - CI/CD pipelines (GitLab CI, GitHub Actions)

#### Server Error Handling

The server fails fast on errors:
- Missing or invalid `complypack.yaml`
- Unsupported platform
- Catalog pull failures
- Duplicate catalog IDs

All errors include actionable remediation guidance.

## Library Quick Start

### Packing a Policy

```go
package main

import (
    "context"
    "strings"

    "github.com/complytime/complypack/pkg/complypack"
    "oras.land/oras-go/v2/content/memory"
)

func main() {
    ctx := context.Background()
    store := memory.New()

    // Configure the artifact
    cfg := complypack.Config{
        EvaluatorID: "io.complytime.opa",
        Version:     "1.0.0",
    }

    // Pack the policy content
    content := strings.NewReader("policy content here")
    desc, err := complypack.Pack(ctx, store, cfg, content)
    if err != nil {
        panic(err)
    }

    // desc.Digest contains the artifact reference
}
```

### Unpacking a Policy

```go
result, err := complypack.Unpack(ctx, store, desc)
if err != nil {
    panic(err)
}
defer result.Content.Close()

// Access configuration
fmt.Printf("Evaluator: %s\n", result.Config.EvaluatorID)

// Read content
content, _ := io.ReadAll(result.Content)
```

### With Signing

```go
// Pack with keyed signing
desc, err := complypack.Pack(ctx, store, cfg, content,
    complypack.WithSigning("/path/to/private.key"))

// Pack with keyless signing (OIDC)
desc, err := complypack.Pack(ctx, store, cfg, content,
    complypack.WithKeylessSigning("user@example.com", "https://accounts.google.com"))

// Unpack with verification
result, err := complypack.Unpack(ctx, store, desc,
    complypack.WithVerification("/path/to/public.key"))
```

### With Provenance

```go
cfg := complypack.Config{
    EvaluatorID: "io.complytime.opa",
    Version:     "1.0.0",
    Source: &complypack.Provenance{
        GemaraContent: "oci://registry/gemara/controls:v1.0.0",
        PolicyID:      "policy-123",
    },
}
```

## Architecture

ComplyPack uses **OCI Image Manifest v1.1** with the following structure:

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "artifactType": "application/vnd.complypack.artifact.v1",
  "config": {
    "mediaType": "application/vnd.complypack.config.v1+json",
    "digest": "sha256:...",
    "size": 324
  },
  "layers": [
    {
      "mediaType": "application/vnd.complypack.content.v1.tar+gzip",
      "digest": "sha256:...",
      "size": 1024000
    }
  ]
}
```

### Media Types

| Purpose       | Media Type                                      |
|---------------|-------------------------------------------------|
| Artifact Type | `application/vnd.complypack.artifact.v1`          |
| Config Layer  | `application/vnd.complypack.config.v1+json`       |
| Content Layer | `application/vnd.complypack.content.v1.tar+gzip`  |

### Content Layer

The content layer is **opaque** - the library treats it as raw bytes. For OPA-based policies, this would typically be an OPA bundle tarball. The `evaluator-id` in the config determines how consumers should interpret the content.

### Memory Usage

Pack() loads the entire content into memory for digest calculation. Content size is limited to **100MB** to prevent memory exhaustion. For larger artifacts, consider alternative approaches or splitting content.

## Error Handling

ComplyPack uses sentinel errors for predictable error checking:

```go
import "errors"

_, err := complypack.Pack(ctx, store, cfg, content)
if errors.Is(err, complypack.ErrEmptyContent) {
    // Handle empty content
}
if errors.Is(err, complypack.ErrContentTooLarge) {
    // Handle content exceeding 100MB limit
}
```

Available sentinel errors:
- `ErrInvalidConfig` - Config validation failed
- `ErrEmptyContent` - Content reader returned zero bytes
- `ErrContentTooLarge` - Content exceeds 100MB limit
- `ErrSigningFailed` - Signing operation failed
- `ErrVerificationFailed` - Signature verification failed
- `ErrInvalidMediaType` - Unexpected media type in manifest
- `ErrNoContentLayer` - Manifest missing content layer

## Storage Backends

ComplyPack works with any ORAS-compatible storage backend:

```go
// In-memory (for testing)
store := memory.New()

// Filesystem
store, err := file.New("/tmp/oci-store")
defer store.Close()

// Remote registry (via ORAS)
repo, err := remote.NewRepository("ghcr.io/org/repo")
store := repo
```

## Current Limitations

- **Signing/Verification**: Validation logic is implemented, but full sigstore-go integration is pending. Signing/verification options will return "not yet implemented" errors.
- **Content Size**: Maximum 100MB per artifact
- **Single Content Layer**: Only one content layer per artifact is supported

## Contributing

Contributions are welcome! Please see our [contributing guidelines](CONTRIBUTING.md).

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.

## Related Projects

- [ORAS](https://oras.land/) - OCI Registry as Storage
- [Sigstore](https://www.sigstore.dev/) - Keyless signing infrastructure
- [Open Policy Agent](https://www.openpolicyagent.org/) - Policy-based control
- [Gemara](https://github.com/gemaraproj/gemara) - Compliance policy framework
