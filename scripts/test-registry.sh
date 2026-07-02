#!/usr/bin/env bash
# test-registry.sh — Start a local OCI registry and push test Gemara bundles.
#
# Usage:
#   ./scripts/test-registry.sh         # start registry + push bundles
#   ./scripts/test-registry.sh stop    # stop and clean up
#
# Prerequisites:
#   - podman (or docker via podman-docker)
#   - go 1.26+
#   - complyctl repo cloned alongside complypack
#
# After running, test with:
#   cd /tmp/complypack-test
#   claude --plugin-dir /path/to/complypack
#   /setup  (source: oci+http://localhost:5050/gemara/test-opa-controls:v1.0.0)

set -euo pipefail

REGISTRY_PORT="${REGISTRY_PORT:-5050}"
REGISTRY_NAME="complypack-test-registry"
REGISTRY_IMAGE="ghcr.io/project-zot/zot-linux-amd64:latest"
COMPLYPACK_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPLYCTL_ROOT="${COMPLYCTL_ROOT:-$(cd "$COMPLYPACK_ROOT/../complyctl" 2>/dev/null && pwd)}"
TESTDATA="${COMPLYCTL_ROOT}/cmd/mock-oci-registry/testdata"
PUSH_BUNDLE_DIR="/tmp/complypack-push-bundle"
TEST_DIR="/tmp/complypack-test"

# --- Stop command ---
if [[ "${1:-}" == "stop" ]]; then
    echo "Stopping registry..."
    podman rm -f "$REGISTRY_NAME" 2>/dev/null || true
    echo "Done."
    exit 0
fi

# --- Validate prerequisites ---
if ! command -v podman &>/dev/null; then
    echo "Error: podman is required. Install it or use podman-docker." >&2
    exit 1
fi

if [[ ! -d "$TESTDATA" ]]; then
    echo "Error: complyctl testdata not found at $TESTDATA" >&2
    echo "Clone complyctl alongside complypack or set COMPLYCTL_ROOT." >&2
    exit 1
fi

# --- Start registry ---
if curl -s "http://localhost:${REGISTRY_PORT}/v2/" &>/dev/null; then
    echo "Registry already running on port ${REGISTRY_PORT}."
else
    echo "Starting zot registry on port ${REGISTRY_PORT}..."
    podman rm -f "$REGISTRY_NAME" 2>/dev/null || true
    podman run -d --name "$REGISTRY_NAME" -p "${REGISTRY_PORT}:5000" "$REGISTRY_IMAGE"
    # Wait for registry to be ready
    for _i in $(seq 1 10); do
        if curl -s "http://localhost:${REGISTRY_PORT}/v2/" &>/dev/null; then
            break
        fi
        sleep 1
    done
    if ! curl -s "http://localhost:${REGISTRY_PORT}/v2/" &>/dev/null; then
        echo "Error: Registry failed to start." >&2
        exit 1
    fi
    echo "Registry started."
fi

# --- Build push-bundle tool ---
if [[ ! -f "$PUSH_BUNDLE_DIR/go.mod" ]]; then
    echo "Building push-bundle tool..."
    mkdir -p "$PUSH_BUNDLE_DIR"
    cat > "$PUSH_BUNDLE_DIR/main.go" << 'GOEOF'
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gemaraproj/go-gemara/bundle"
	"oras.land/oras-go/v2/registry/remote"
)

func main() {
	if len(os.Args) < 4 {
		log.Fatalf("usage: %s <registry/repo> <tag> <file1.yaml> [file2.yaml ...]", os.Args[0])
	}

	repoRef := os.Args[1]
	tag := os.Args[2]
	files := os.Args[3:]

	var bundleFiles []bundle.File
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("reading %s: %v", path, err)
		}
		bundleFiles = append(bundleFiles, bundle.File{
			Name: filepath.Base(path),
			Data: data,
		})
	}

	b := &bundle.Bundle{
		Manifest: bundle.Manifest{
			BundleVersion: "1.0.0",
			GemaraVersion: "1.0.0",
		},
		Files: bundleFiles,
	}

	ctx := context.Background()
	repo, err := remote.NewRepository(repoRef)
	if err != nil {
		log.Fatalf("creating repository: %v", err)
	}
	repo.PlainHTTP = true

	desc, err := bundle.Pack(ctx, repo, b)
	if err != nil {
		log.Fatalf("packing bundle: %v", err)
	}

	err = repo.Tag(ctx, desc, tag)
	if err != nil {
		log.Fatalf("tagging: %v", err)
	}

	fmt.Printf("Pushed %s:%s (%s)\n", repoRef, tag, desc.Digest)
}
GOEOF
    cd "$PUSH_BUNDLE_DIR"
    go mod init push-bundle
    go mod tidy
fi

# --- Push test bundles ---
echo "Pushing test bundles..."

cd "$PUSH_BUNDLE_DIR"

# OPA container security catalog
go run . "localhost:${REGISTRY_PORT}/gemara/test-opa-controls" v1.0.0 \
    "$TESTDATA/test-opa-catalog.yaml"

# Branch protection catalog
go run . "localhost:${REGISTRY_PORT}/gemara/test-branch-protection" v1.0.0 \
    "$TESTDATA/test-branch-protection-catalog.yaml"

# --- Set up test directory ---
mkdir -p "$TEST_DIR"
cat > "$TEST_DIR/.mcp.json" << MCPEOF
{
  "mcpServers": {
    "complypack": {
      "command": "go",
      "args": ["run", "${COMPLYPACK_ROOT}/cmd/complypack",
               "mcp", "serve",
               "--source", "oci+http://localhost:${REGISTRY_PORT}/gemara/test-opa-controls:v1.0.0",
               "--schema", "kubernetes-deployment"]
    }
  }
}
MCPEOF

echo ""
echo "=== Ready ==="
echo ""
echo "Registry: http://localhost:${REGISTRY_PORT}"
echo "Test dir:  ${TEST_DIR}"
echo ""
echo "Available sources:"
echo "  oci+http://localhost:${REGISTRY_PORT}/gemara/test-opa-controls:v1.0.0"
echo "  oci+http://localhost:${REGISTRY_PORT}/gemara/test-branch-protection:v1.0.0"
echo ""
echo "To test:"
echo "  cd ${TEST_DIR}"
echo "  claude --plugin-dir ${COMPLYPACK_ROOT}"
echo ""
echo "Commands to try:"
echo "  /setup"
echo "  /pack generate policy for container-run-as-nonroot"
echo "  /pipeline"
echo ""
echo "To stop: $0 stop"
