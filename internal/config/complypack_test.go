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

	configContent := `evaluator-id: io.complytime.opa
version: 0.1.0
gemara:
  source: catalogs/nist-800-53.yaml
schemas:
  - path: schemas/kubernetes.cue
    platform: kubernetes
  - path: schemas/terraform.cue
    platform: terraform
policies:
  dir: policies/
  helpers:
    - policies/helpers.rego
tests:
  dir: tests/
fixtures:
  dir: fixtures/
output:
  dir: dist/
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "io.complytime.opa", config.EvaluatorID)
	assert.Equal(t, "0.1.0", config.Version)
	assert.Equal(t, "catalogs/nist-800-53.yaml", config.Gemara.Source)
	assert.Len(t, config.Schemas, 2)
	assert.Equal(t, "schemas/kubernetes.cue", config.Schemas[0].Path)
	assert.Equal(t, "kubernetes", config.Schemas[0].Platform)
	assert.NotNil(t, config.Policies)
	assert.Equal(t, "policies/", config.Policies.Dir)
	assert.Len(t, config.Policies.Helpers, 1)
	assert.NotNil(t, config.Tests)
	assert.Equal(t, "tests/", config.Tests.Dir)
}

func TestLoadConfig_MinimalConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `evaluator-id: io.complytime.opa
version: 0.1.0
gemara:
  source: catalogs/controls.yaml
schemas:
  - path: schemas/kubernetes.cue
    platform: kubernetes
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "io.complytime.opa", config.EvaluatorID)
	assert.Equal(t, "0.1.0", config.Version)
	assert.Equal(t, "catalogs/controls.yaml", config.Gemara.Source)
	assert.Len(t, config.Schemas, 1)
	assert.Nil(t, config.Policies)
	assert.Nil(t, config.Tests)
}

func TestLoadConfig_MissingEvaluatorID(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `version: 0.1.0
gemara:
  source: catalogs/controls.yaml
schemas:
  - path: schemas/kubernetes.cue
    platform: kubernetes
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "evaluator-id")
}

func TestLoadConfig_MissingVersion(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `evaluator-id: io.complytime.opa
gemara:
  source: catalogs/controls.yaml
schemas:
  - path: schemas/kubernetes.cue
    platform: kubernetes
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "version")
}

func TestLoadConfig_MissingGemaraSource(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `evaluator-id: io.complytime.opa
version: 0.1.0
schemas:
  - path: schemas/kubernetes.cue
    platform: kubernetes
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "gemara.source")
}

func TestLoadConfig_MissingSchemas(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `evaluator-id: io.complytime.opa
version: 0.1.0
gemara:
  source: catalogs/controls.yaml
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "schemas")
}

func TestLoadConfig_SchemaMissingPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `evaluator-id: io.complytime.opa
version: 0.1.0
gemara:
  source: catalogs/controls.yaml
schemas:
  - platform: kubernetes
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "path")
}

func TestLoadConfig_SchemaMissingPlatform(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `evaluator-id: io.complytime.opa
version: 0.1.0
gemara:
  source: catalogs/controls.yaml
schemas:
  - path: schemas/kubernetes.cue
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "platform")
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	config, err := LoadConfig("/nonexistent/path/complypack.yaml")
	assert.Error(t, err)
	assert.Nil(t, config)
}
