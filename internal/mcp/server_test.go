// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	ctx := context.Background()

	t.Run("success with valid catalogs", func(t *testing.T) {
		// Create temp cache directory
		cacheDir := t.TempDir()

		// Create mock OCI store with catalog bundles
		ociStore := t.TempDir()
		createMockCatalogBundle(t, ociStore, "controls-v1", map[string]string{
			"catalog.yaml": mockControlsCatalog,
		})

		// Create config file (no schema source = use embedded)
		configPath := filepath.Join(t.TempDir(), "complypack.yaml")
		configYAML := `evaluator-id: opa
version: 0.1.0
gemara:
  source: ` + filepath.Join(ociStore, "controls-v1", "catalog.yaml") + `
schemas:
  - platform: kubernetes-deployment
`
		err := os.WriteFile(configPath, []byte(configYAML), 0600)
		require.NoError(t, err)

		// Create server
		srv, err := NewServer(ctx, &ServerOptions{
			ConfigPath: configPath,
			OCIStore:   ociStore,
			CacheDir:   cacheDir,
		})

		require.NoError(t, err)
		require.NotNil(t, srv)

		// Verify resource store has catalog
		store := srv.ResourceStore
		require.NotNil(t, store)
		assert.Len(t, store.artifacts, 1)
		assert.Contains(t, store.artifacts, "controls-v1")

		// Verify schemas loaded
		assert.NotEmpty(t, store.schemas)
		assert.Contains(t, store.schemas, "kubernetes-deployment")
	})

	t.Run("error when catalog missing", func(t *testing.T) {
		cacheDir := t.TempDir()
		ociStore := t.TempDir()

		configPath := filepath.Join(t.TempDir(), "complypack.yaml")
		configYAML := `evaluator-id: opa
version: 0.1.0
gemara:
  source: /nonexistent/catalog.yaml
schemas:
  - path: schemas/kubernetes.cue
    platform: kubernetes
`
		err := os.WriteFile(configPath, []byte(configYAML), 0600)
		require.NoError(t, err)

		srv, err := NewServer(ctx, &ServerOptions{
			ConfigPath: configPath,
			OCIStore:   ociStore,
			CacheDir:   cacheDir,
		})

		assert.Error(t, err)
		assert.Nil(t, srv)
		assert.Contains(t, err.Error(), "failed to load artifacts")
	})

	t.Run("fail fast when configured schema source cannot be loaded", func(t *testing.T) {
		cacheDir := t.TempDir()
		ociStore := t.TempDir()

		createMockCatalogBundle(t, ociStore, "controls-v1", map[string]string{
			"catalog.yaml": mockControlsCatalog,
		})

		configPath := filepath.Join(t.TempDir(), "complypack.yaml")
		configYAML := `evaluator-id: opa
version: 0.1.0
gemara:
  source: ` + filepath.Join(ociStore, "controls-v1", "catalog.yaml") + `
schemas:
  - path: schemas/invalid.cue
    platform: unsupported-platform
`
		err := os.WriteFile(configPath, []byte(configYAML), 0600)
		require.NoError(t, err)

		srv, err := NewServer(ctx, &ServerOptions{
			ConfigPath: configPath,
			OCIStore:   ociStore,
			CacheDir:   cacheDir,
		})

		assert.Error(t, err)
		assert.Nil(t, srv)
		assert.Contains(t, err.Error(), "failed to load schemas")
	})

	// Removed: duplicate catalog test - no longer applicable with single source config
}

func TestLoadSchemas(t *testing.T) {
	ctx := context.Background()

	t.Run("resolves index defaults for known platform", func(t *testing.T) {
		refs := []config.SchemaRef{
			{Platform: "ci-github-actions"},
			{Platform: "kubernetes-deployment"},
		}

		schemaMap, cueSchemaMap, err := loadSchemas(ctx, refs, schema.DefaultRegistry())
		require.NoError(t, err)
		assert.Contains(t, schemaMap, "ci-github-actions")
		assert.NotEmpty(t, schemaMap["ci-github-actions"])
		assert.Contains(t, cueSchemaMap, "ci-github-actions")
		assert.True(t, cueSchemaMap["ci-github-actions"].Exists())
		assert.Contains(t, schemaMap, "kubernetes-deployment")
		assert.NotEmpty(t, schemaMap["kubernetes-deployment"])
		assert.Contains(t, cueSchemaMap, "kubernetes-deployment")
		assert.True(t, cueSchemaMap["kubernetes-deployment"].Exists())
	})

	t.Run("skips unknown platform with no source", func(t *testing.T) {
		refs := []config.SchemaRef{
			{Platform: "unknown-platform"},
		}

		schemaMap, _, err := loadSchemas(ctx, refs, schema.DefaultRegistry())
		require.NoError(t, err)
		assert.NotContains(t, schemaMap, "unknown-platform")
	})

	t.Run("explicit source overrides index default", func(t *testing.T) {
		refs := []config.SchemaRef{
			{Platform: "ci-github-actions", Source: "cue://cue.dev/x/githubactions@v0#Workflow"},
		}

		schemaMap, cueSchemaMap, err := loadSchemas(ctx, refs, schema.DefaultRegistry())
		require.NoError(t, err)
		assert.Contains(t, schemaMap, "ci-github-actions")
		assert.True(t, cueSchemaMap["ci-github-actions"].Exists())
	})
}

// createMockCatalogBundle creates a directory with catalog files for testing.
func createMockCatalogBundle(t *testing.T, baseDir, bundleName string, files map[string]string) {
	t.Helper()

	bundleDir := filepath.Join(baseDir, bundleName)
	err := os.MkdirAll(bundleDir, 0755)
	require.NoError(t, err)

	for filename, content := range files {
		path := filepath.Join(bundleDir, filename)
		err := os.WriteFile(path, []byte(content), 0600)
		require.NoError(t, err)
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

// mockControlsCatalogV2 is a second catalog with a different metadata.id for multi-source testing.
const mockControlsCatalogV2 = `metadata:
  id: controls-v2
  type: ControlCatalog
  gemara-version: "1.0.0"
controls:
  - id: SC-1
    title: System Communications Protection
    description: Protect system communications.
`

func TestNewServer_MultiSource(t *testing.T) {
	ctx := context.Background()

	cacheDir := t.TempDir()
	ociStore := t.TempDir()

	// Create two separate catalog files with different metadata.id values
	createMockCatalogBundle(t, ociStore, "source-a", map[string]string{
		"catalog.yaml": mockControlsCatalog,
	})
	createMockCatalogBundle(t, ociStore, "source-b", map[string]string{
		"catalog.yaml": mockControlsCatalogV2,
	})

	sourceA := filepath.Join(ociStore, "source-a", "catalog.yaml")
	sourceB := filepath.Join(ociStore, "source-b", "catalog.yaml")

	configPath := filepath.Join(t.TempDir(), "complypack.yaml")
	configYAML := `evaluator-id: opa
version: 0.1.0
gemara:
  sources:
    - source: ` + sourceA + `
    - source: ` + sourceB + `
schemas:
  - platform: kubernetes-deployment
`
	err := os.WriteFile(configPath, []byte(configYAML), 0600)
	require.NoError(t, err)

	srv, err := NewServer(ctx, &ServerOptions{
		ConfigPath: configPath,
		OCIStore:   ociStore,
		CacheDir:   cacheDir,
	})

	require.NoError(t, err)
	require.NotNil(t, srv)

	store := srv.ResourceStore
	require.NotNil(t, store)
	assert.Len(t, store.artifacts, 2)
	assert.Contains(t, store.artifacts, "controls-v1")
	assert.Contains(t, store.artifacts, "controls-v2")
}
