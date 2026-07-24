// SPDX-License-Identifier: Apache-2.0

package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsOCIReference(t *testing.T) {
	tests := []struct {
		source string
		want   bool
	}{
		// Positive cases: OCI references with tags
		{"ghcr.io/org/catalog:v1", true},
		{"localhost:5000/repo:tag", true},
		{"ghcr.io/org/repo:latest", true},
		{"registry.example.com/org/repo:v1.0.0", true},
		// Scheme-prefixed references with multi-segment paths
		{"http://registry.io/org/repo", true},
		{"https://registry.io/org/image", true},

		// Positive cases: tagless OCI references (issue #136)
		{"ghcr.io/org/catalog", true},
		{"registry.example.com/org/repo", true},
		{"localhost/repo", true},

		// Positive cases: IP-based registries
		{"192.168.1.1/team/catalog", true},
		{"192.168.1.1:5000/team/catalog", true},

		// Negative cases: not OCI references
		{"catalog.yaml", false},
		{"/absolute/path/to/file.yaml", false},
		{"relative/path.yaml", false},
		{"", false},
		{"single-word", false},

		// Edge cases: file paths with dots that should NOT match
		{"v1.2/config.yaml", false},
		{"./local/path", false},
		{"mydir/file", false},

		// Edge cases: DNS-like directory names (PR #190 review feedback)
		{"my.project/config.yaml", false},
		{"data.backup/controls.json", false},

		// Edge case: bare-host scheme URL without dot (PR #190 review feedback)
		{"http://registry/repo", false},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			got := IsOCIReference(tt.source)
			assert.Equal(t, tt.want, got)
		})
	}
}

// mockControlsCatalog is a minimal valid Gemara control catalog for testing.
const mockControlsCatalog = `metadata:
  id: controls-v1
  type: ControlCatalog
  gemara-version: "1.0.0"
controls:
  - id: AC-1
    title: Access Control Policy
    description: Develop and maintain access control policy.
`

func TestLoadArtifacts_FileSource(t *testing.T) {
	ctx := context.Background()

	t.Run("loads file with file:// prefix", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "catalog.yaml")
		err := os.WriteFile(path, []byte(mockControlsCatalog), 0600)
		require.NoError(t, err)

		result, err := LoadArtifacts(ctx, "file://"+path, false, "")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Contains(t, result.Catalogs, "controls-v1")
	})

	t.Run("loads bare file path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "catalog.yaml")
		err := os.WriteFile(path, []byte(mockControlsCatalog), 0600)
		require.NoError(t, err)

		result, err := LoadArtifacts(ctx, path, false, "")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Contains(t, result.Catalogs, "controls-v1")
	})

	t.Run("errors on missing file", func(t *testing.T) {
		_, err := LoadArtifacts(ctx, "/nonexistent/path.yaml", false, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read file")
	})

	t.Run("errors on missing file with file:// prefix", func(t *testing.T) {
		_, err := LoadArtifacts(ctx, "file:///nonexistent/path.yaml", false, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read file")
	})
}

func TestLoadArtifacts_OCISourceRequiresRegistry(t *testing.T) {
	ctx := context.Background()

	// OCI sources require a registry, which is not available in unit tests.
	// Verify they are routed correctly by checking the error type --
	// an OCI source should fail with a registry/network error, not a file error.

	t.Run("oci:// prefix routes to bundle loader", func(t *testing.T) {
		_, err := LoadArtifacts(ctx, "oci://localhost:9999/nonexistent:v1", false, "")
		require.Error(t, err)
		// Should NOT contain "failed to read file" -- that would mean it was
		// routed to the file loader instead of the bundle loader.
		assert.NotContains(t, err.Error(), "failed to read file")
	})

	t.Run("bare OCI reference routes to bundle loader", func(t *testing.T) {
		_, err := LoadArtifacts(ctx, "ghcr.io/nonexistent/repo:v1", false, "")
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "failed to read file")
	})

	t.Run("tagless OCI reference routes to bundle loader", func(t *testing.T) {
		_, err := LoadArtifacts(ctx, "ghcr.io/nonexistent/catalog", false, "")
		require.Error(t, err)
		// Should NOT contain "failed to read file" -- that would mean it was
		// routed to the file loader instead of the bundle loader (issue #136).
		assert.NotContains(t, err.Error(), "failed to read file")
	})
}

func TestLoadBundleArtifacts_InMemoryFallback(t *testing.T) {
	ctx := context.Background()

	// When cacheDir is empty, loadBundleArtifacts should use memory.New()
	// instead of an on-disk OCI store. We verify this by passing an empty
	// cacheDir and confirming the error is a registry/network error (meaning
	// the code got past the store selection without erroring on cache setup).

	t.Run("empty cacheDir uses in-memory store", func(t *testing.T) {
		_, err := loadBundleArtifacts(ctx, "localhost:9999/nonexistent:v1", false, "")
		require.Error(t, err)
		// The error should be about the registry pull failing, NOT about
		// failing to open a cache store. This proves memory.New() was used.
		assert.NotContains(t, err.Error(), "failed to open cache store")
		assert.Contains(t, err.Error(), "failed to pull from registry")
	})

	t.Run("non-empty cacheDir uses on-disk store", func(t *testing.T) {
		cacheDir := t.TempDir()
		_, err := loadBundleArtifacts(ctx, "localhost:9999/nonexistent:v1", false, cacheDir)
		require.Error(t, err)
		// Same registry error expected -- the store was created successfully
		// but the pull fails because there's no registry.
		assert.NotContains(t, err.Error(), "failed to open cache store")
		assert.Contains(t, err.Error(), "failed to pull from registry")

		// Verify the OCI store was actually created on disk
		_, statErr := os.Stat(filepath.Join(cacheDir, "index.json"))
		assert.NoError(t, statErr, "OCI layout index.json should exist in cache dir")
	})
}
