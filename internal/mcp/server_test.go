// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/schemas"
	"github.com/gemaraproj/go-gemara"
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
  - platform: kubernetes
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
		assert.Len(t, store.catalogs, 1)
		assert.Contains(t, store.catalogs, "controls-v1")

		// Verify schemas loaded
		assert.NotEmpty(t, store.schemas)
		assert.Contains(t, store.schemas, "kubernetes")
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

func TestExtractCatalogName(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected string
		wantErr  bool
	}{
		{
			name: "from metadata.id",
			yaml: `metadata:
  id: my-catalog
  version: 1.0`,
			expected: "my-catalog",
			wantErr:  false,
		},
		{
			name: "missing metadata.id",
			yaml: `metadata:
  version: 1.0`,
			expected: "",
			wantErr:  true,
		},
		{
			name:     "invalid YAML",
			yaml:     `invalid: [unclosed`,
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractCatalogName([]byte(tt.yaml))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestLoadSchemas(t *testing.T) {
	ctx := context.Background()

	// Create schema refs for all built-in platforms (no source = use embedded)
	var refs []config.SchemaRef
	for _, platform := range schemas.BuiltInPlatforms {
		refs = append(refs, config.SchemaRef{Platform: platform})
	}

	t.Run("loads all built-in schemas with CUE values", func(t *testing.T) {
		schemaMap, cueSchemaMap, err := loadSchemas(ctx, refs)
		require.NoError(t, err)
		require.NotNil(t, schemaMap)
		require.NotNil(t, cueSchemaMap)

		for _, platform := range schemas.BuiltInPlatforms {
			assert.Contains(t, schemaMap, platform, "missing schema for platform %s", platform)
			assert.NotEmpty(t, schemaMap[platform])
			assert.Contains(t, cueSchemaMap, platform, "missing CUE schema for platform %s", platform)
			assert.True(t, cueSchemaMap[platform].Exists(), "CUE schema for %s should exist", platform)
		}
	})

	t.Run("schema content is valid JSON", func(t *testing.T) {
		schemaMap, _, err := loadSchemas(ctx, refs)
		require.NoError(t, err)

		for platform, data := range schemaMap {
			assert.NotEmpty(t, data, "empty schema for platform %s", platform)
			trimmed := strings.TrimSpace(string(data))
			assert.True(t, strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "["),
				"schema for %s doesn't look like JSON", platform)
		}
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

func TestLoadedArtifacts_Merge(t *testing.T) {
	a := &LoadedArtifacts{
		RawCatalogs:       map[string][]byte{"cat-a": []byte("a")},
		Catalogs:          map[string]*gemara.ControlCatalog{"cat-a": {}},
		Policies:          map[string]*gemara.Policy{},
		EffectivePolicies: map[string]*gemara.EffectivePolicy{},
	}
	b := &LoadedArtifacts{
		RawCatalogs:       map[string][]byte{"cat-b": []byte("b")},
		Catalogs:          map[string]*gemara.ControlCatalog{"cat-b": {}},
		Policies:          map[string]*gemara.Policy{},
		EffectivePolicies: map[string]*gemara.EffectivePolicy{},
	}

	err := a.Merge(b)
	require.NoError(t, err)
	assert.Len(t, a.RawCatalogs, 2)
	assert.Len(t, a.Catalogs, 2)
}

func TestLoadedArtifacts_MergeDuplicateID(t *testing.T) {
	a := &LoadedArtifacts{
		RawCatalogs:       map[string][]byte{"same-id": []byte("a")},
		Catalogs:          map[string]*gemara.ControlCatalog{},
		Policies:          map[string]*gemara.Policy{},
		EffectivePolicies: map[string]*gemara.EffectivePolicy{},
	}
	b := &LoadedArtifacts{
		RawCatalogs:       map[string][]byte{"same-id": []byte("b")},
		Catalogs:          map[string]*gemara.ControlCatalog{},
		Policies:          map[string]*gemara.Policy{},
		EffectivePolicies: map[string]*gemara.EffectivePolicy{},
	}

	err := a.Merge(b)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "same-id")
}

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
  - platform: kubernetes
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
	assert.Len(t, store.catalogs, 2)
	assert.Contains(t, store.catalogs, "controls-v1")
	assert.Contains(t, store.catalogs, "controls-v2")
}
