package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_ValidConfigWithAllFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `platform: oscal
gemara-catalogs:
  - name: nist-800-53
    path: ./catalogs/nist-800-53.yaml
  - name: custom-controls
    path: ./catalogs/custom.yaml
platform-schemas:
  - name: component-definition
    path: ./schemas/component-definition.json
  - name: ssp
    path: ./schemas/ssp.json
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "oscal", config.Platform)
	assert.Len(t, config.GemaraCatalogs, 2)
	assert.Equal(t, "nist-800-53", config.GemaraCatalogs[0].Name)
	assert.Equal(t, "./catalogs/nist-800-53.yaml", config.GemaraCatalogs[0].Path)
	assert.Len(t, config.PlatformSchemas, 2)
	assert.Equal(t, "component-definition", config.PlatformSchemas[0].Name)
	assert.Equal(t, "./schemas/component-definition.json", config.PlatformSchemas[0].Path)
}

func TestLoadConfig_MinimalConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `platform: oscal
gemara-catalogs:
  - name: nist-800-53
    path: ./catalogs/nist-800-53.yaml
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "oscal", config.Platform)
	assert.Len(t, config.GemaraCatalogs, 1)
	assert.Empty(t, config.PlatformSchemas)
}

func TestLoadConfig_MissingPlatform(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `gemara-catalogs:
  - name: nist-800-53
    path: ./catalogs/nist-800-53.yaml
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "platform")
}

func TestLoadConfig_MissingCatalogs(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `platform: oscal
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "gemara-catalogs")
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	config, err := LoadConfig("/nonexistent/path/complypack.yaml")
	assert.Error(t, err)
	assert.Nil(t, config)
}
