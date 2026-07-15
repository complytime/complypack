// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLoadConfig_ValidConfigWithAllFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `evaluator-id: opa
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

	config, err := LoadConfig(configPath, false, os.Stderr)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "opa", config.EvaluatorID)
	assert.Equal(t, "0.1.0", config.Version)
	assert.Equal(t, "catalogs/nist-800-53.yaml", config.Gemara.Sources[0].Source)
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

	configContent := `evaluator-id: opa
version: 0.1.0
gemara:
  source: catalogs/controls.yaml
schemas:
  - path: schemas/kubernetes.cue
    platform: kubernetes
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	config, err := LoadConfig(configPath, false, os.Stderr)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "opa", config.EvaluatorID)
	assert.Equal(t, "0.1.0", config.Version)
	assert.Equal(t, "catalogs/controls.yaml", config.Gemara.Sources[0].Source)
	assert.Len(t, config.Schemas, 1)
	assert.Nil(t, config.Policies)
	assert.Nil(t, config.Tests)
}

func TestLoadConfig_OptionalEvaluatorID(t *testing.T) {
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

	config, err := LoadConfig(configPath, false, os.Stderr)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Empty(t, config.EvaluatorID)
}

func TestLoadConfig_MissingVersion(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `evaluator-id: opa
gemara:
  source: catalogs/controls.yaml
schemas:
  - path: schemas/kubernetes.cue
    platform: kubernetes
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	config, err := LoadConfig(configPath, false, os.Stderr)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "version")
}

func TestValidateForMCP_MissingGemaraSource(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `evaluator-id: opa
version: 0.1.0
schemas:
  - path: schemas/kubernetes.cue
    platform: kubernetes
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath, false, os.Stderr)
	require.NoError(t, err)

	err = cfg.ValidateForMCP()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gemara.sources")
}

func TestValidateForMCP_MissingSchemas(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `evaluator-id: opa
version: 0.1.0
gemara:
  source: catalogs/controls.yaml
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath, false, os.Stderr)
	require.NoError(t, err)

	err = cfg.ValidateForMCP()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schemas")
}

func TestLoadConfig_SchemaMissingPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `evaluator-id: opa
version: 0.1.0
gemara:
  source: catalogs/controls.yaml
schemas:
  - platform: kubernetes
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	// Schema without source/path is valid - uses embedded schema
	config, err := LoadConfig(configPath, false, os.Stderr)
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "kubernetes", config.Schemas[0].Platform)
}

func TestLoadConfig_SchemaMissingPlatform(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `evaluator-id: opa
version: 0.1.0
gemara:
  source: catalogs/controls.yaml
schemas:
  - path: schemas/kubernetes.cue
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	config, err := LoadConfig(configPath, false, os.Stderr)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "platform")
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	config, err := LoadConfig("/nonexistent/path/complypack.yaml", false, os.Stderr)
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestLoadConfig_MultiSourceGemara(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `version: 0.1.0
gemara:
  sources:
    - source: catalogs/nist-800-53.yaml
    - source: catalogs/iso-42001.yaml
    - source: ghcr.io/org/controls:v1
      plain-http: true
schemas:
  - platform: kubernetes
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	config, err := LoadConfig(configPath, false, os.Stderr)
	require.NoError(t, err)
	require.Len(t, config.Gemara.Sources, 3)
	assert.Equal(t, "catalogs/nist-800-53.yaml", config.Gemara.Sources[0].Source)
	assert.Equal(t, "catalogs/iso-42001.yaml", config.Gemara.Sources[1].Source)
	assert.Equal(t, "ghcr.io/org/controls:v1", config.Gemara.Sources[2].Source)
	assert.False(t, config.Gemara.Sources[0].PlainHTTP)
	assert.True(t, config.Gemara.Sources[2].PlainHTTP)
}

func TestLoadConfig_LegacySingleSourceBackcompat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `version: 0.1.0
gemara:
  source: catalogs/controls.yaml
  plain-http: true
schemas:
  - platform: kubernetes
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	config, err := LoadConfig(configPath, false, os.Stderr)
	require.NoError(t, err)
	require.Len(t, config.Gemara.Sources, 1)
	assert.Equal(t, "catalogs/controls.yaml", config.Gemara.Sources[0].Source)
	assert.True(t, config.Gemara.Sources[0].PlainHTTP)
}

func TestValidateForInit_Valid(t *testing.T) {
	cfg := &ComplyPackConfig{
		ID:          "io.complytime.test",
		EvaluatorID: "opa",
		Version:     "1.0.0",
		Schemas:     []SchemaRef{{Platform: "kubernetes"}},
		Gemara:      GemaraConfig{Sources: []GemaraSourceEntry{{Source: "catalogs/controls.yaml"}}},
	}
	assert.NoError(t, cfg.ValidateForInit())
}

func TestValidateForInit_MissingID(t *testing.T) {
	cfg := &ComplyPackConfig{
		EvaluatorID: "opa",
		Version:     "1.0.0",
		Schemas:     []SchemaRef{{Platform: "kubernetes"}},
		Gemara:      GemaraConfig{Sources: []GemaraSourceEntry{{Source: "catalogs/controls.yaml"}}},
	}
	err := cfg.ValidateForInit()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestValidateForInit_MissingEvaluatorID(t *testing.T) {
	cfg := &ComplyPackConfig{
		ID:      "io.complytime.test",
		Version: "1.0.0",
		Schemas: []SchemaRef{{Platform: "kubernetes"}},
		Gemara:  GemaraConfig{Sources: []GemaraSourceEntry{{Source: "catalogs/controls.yaml"}}},
	}
	err := cfg.ValidateForInit()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "evaluator-id")
}

func TestValidateForInit_MissingVersion(t *testing.T) {
	cfg := &ComplyPackConfig{
		ID:          "io.complytime.test",
		EvaluatorID: "opa",
		Schemas:     []SchemaRef{{Platform: "kubernetes"}},
		Gemara:      GemaraConfig{Sources: []GemaraSourceEntry{{Source: "catalogs/controls.yaml"}}},
	}
	err := cfg.ValidateForInit()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestValidateForInit_EmptySchemas(t *testing.T) {
	cfg := &ComplyPackConfig{
		ID:          "io.complytime.test",
		EvaluatorID: "opa",
		Version:     "1.0.0",
		Gemara:      GemaraConfig{Sources: []GemaraSourceEntry{{Source: "catalogs/controls.yaml"}}},
	}
	err := cfg.ValidateForInit()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schemas")
}

func TestValidateForInit_SchemaMissingPlatform(t *testing.T) {
	cfg := &ComplyPackConfig{
		ID:          "io.complytime.test",
		EvaluatorID: "opa",
		Version:     "1.0.0",
		Schemas:     []SchemaRef{{Source: "cue://example.com/k8s"}},
		Gemara:      GemaraConfig{Sources: []GemaraSourceEntry{{Source: "catalogs/controls.yaml"}}},
	}
	err := cfg.ValidateForInit()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "platform")
}

func TestValidateForInit_MissingGemaraSources(t *testing.T) {
	cfg := &ComplyPackConfig{
		ID:          "io.complytime.test",
		EvaluatorID: "opa",
		Version:     "1.0.0",
		Schemas:     []SchemaRef{{Platform: "kubernetes"}},
	}
	err := cfg.ValidateForInit()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gemara.sources")
}

func TestValidateForPack_Valid(t *testing.T) {
	cfg := &ComplyPackConfig{
		ID:          "io.complytime.test",
		EvaluatorID: "opa",
		Version:     "1.0.0",
		Schemas:     []SchemaRef{{Platform: "kubernetes"}},
	}
	assert.NoError(t, cfg.ValidateForPack())
}

func TestValidateForPack_MissingVersion(t *testing.T) {
	cfg := &ComplyPackConfig{
		ID:          "io.complytime.test",
		EvaluatorID: "opa",
		Schemas:     []SchemaRef{{Platform: "kubernetes"}},
	}
	err := cfg.ValidateForPack()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestValidateForPack_MissingID(t *testing.T) {
	cfg := &ComplyPackConfig{
		EvaluatorID: "opa",
		Version:     "1.0.0",
		Schemas:     []SchemaRef{{Platform: "kubernetes"}},
	}
	err := cfg.ValidateForPack()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestValidateForPack_MissingEvaluatorID(t *testing.T) {
	cfg := &ComplyPackConfig{
		ID:      "io.complytime.test",
		Version: "1.0.0",
		Schemas: []SchemaRef{{Platform: "kubernetes"}},
	}
	err := cfg.ValidateForPack()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "evaluator-id")
}

func TestValidate_FormatChecks(t *testing.T) {
	t.Run("invalid version format", func(t *testing.T) {
		cfg := &ComplyPackConfig{Version: "1"}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid version")
	})

	t.Run("invalid id format when present", func(t *testing.T) {
		cfg := &ComplyPackConfig{
			ID:      "NOT VALID",
			Version: "1.0.0",
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid id")
	})

	t.Run("invalid evaluator-id format when present", func(t *testing.T) {
		cfg := &ComplyPackConfig{
			EvaluatorID: "BAD EVAL",
			Version:     "1.0.0",
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid evaluator-id")
	})

	t.Run("absent optional fields pass", func(t *testing.T) {
		cfg := &ComplyPackConfig{Version: "1.0.0"}
		err := cfg.Validate()
		require.NoError(t, err)
	})
}

func TestValidateForPack_InvalidIDFormat(t *testing.T) {
	cfg := &ComplyPackConfig{
		ID:          "not valid",
		EvaluatorID: "opa",
		Version:     "1.0.0",
		Schemas:     []SchemaRef{{Platform: "kubernetes"}},
	}
	err := cfg.ValidateForPack()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid id")
}

func TestValidateForPack_InvalidEvaluatorIDFormat(t *testing.T) {
	cfg := &ComplyPackConfig{
		ID:          "io.complytime.test",
		EvaluatorID: "BAD EVAL",
		Version:     "1.0.0",
		Schemas:     []SchemaRef{{Platform: "kubernetes"}},
	}
	err := cfg.ValidateForPack()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid evaluator-id")
}

func TestValidateForMCP_FormatChecks(t *testing.T) {
	base := ComplyPackConfig{
		Schemas: []SchemaRef{{Platform: "kubernetes"}},
		Gemara:  GemaraConfig{Sources: []GemaraSourceEntry{{Source: "catalogs/controls.yaml"}}},
	}

	t.Run("valid with no optional identity fields", func(t *testing.T) {
		cfg := base
		assert.NoError(t, cfg.ValidateForMCP())
	})

	t.Run("valid with well-formed optional fields", func(t *testing.T) {
		cfg := base
		cfg.ID = "io.complytime.test"
		cfg.EvaluatorID = "opa"
		cfg.Version = "1.0.0"
		assert.NoError(t, cfg.ValidateForMCP())
	})

	t.Run("rejects bad id format when present", func(t *testing.T) {
		cfg := base
		cfg.ID = "NOT VALID"
		err := cfg.ValidateForMCP()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid id")
	})

	t.Run("rejects bad evaluator-id format when present", func(t *testing.T) {
		cfg := base
		cfg.EvaluatorID = "BAD EVAL"
		err := cfg.ValidateForMCP()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid evaluator-id")
	})

	t.Run("rejects bad version format when present", func(t *testing.T) {
		cfg := base
		cfg.Version = "latest"
		err := cfg.ValidateForMCP()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid version")
	})
}

func TestValidateForInit_InvalidFormats(t *testing.T) {
	base := ComplyPackConfig{
		ID:          "io.complytime.test",
		EvaluatorID: "opa",
		Version:     "1.0.0",
		Schemas:     []SchemaRef{{Platform: "kubernetes"}},
		Gemara:      GemaraConfig{Sources: []GemaraSourceEntry{{Source: "catalogs/controls.yaml"}}},
	}

	t.Run("invalid id", func(t *testing.T) {
		cfg := base
		cfg.ID = "singleword"
		err := cfg.ValidateForInit()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid id")
	})

	t.Run("invalid evaluator-id", func(t *testing.T) {
		cfg := base
		cfg.EvaluatorID = "HAS SPACES"
		err := cfg.ValidateForInit()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid evaluator-id")
	})

	t.Run("invalid version", func(t *testing.T) {
		cfg := base
		cfg.Version = "latest"
		err := cfg.ValidateForInit()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid version")
	})
}

func TestValidateForMCP_Valid(t *testing.T) {
	cfg := &ComplyPackConfig{
		Schemas: []SchemaRef{{Platform: "kubernetes"}},
		Gemara:  GemaraConfig{Sources: []GemaraSourceEntry{{Source: "catalogs/controls.yaml"}}},
	}
	assert.NoError(t, cfg.ValidateForMCP())
}

func TestValidateForMCP_SchemaMissingPlatform(t *testing.T) {
	cfg := &ComplyPackConfig{
		Schemas: []SchemaRef{{Source: "cue://example.com/k8s"}},
		Gemara:  GemaraConfig{Sources: []GemaraSourceEntry{{Source: "catalogs/controls.yaml"}}},
	}
	err := cfg.ValidateForMCP()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "platform")
}

func TestMarshalConfigYAML(t *testing.T) {
	cfg := &ComplyPackConfig{
		ID:          "io.example.my-pack",
		EvaluatorID: "opa",
		Version:     "0.1.0",
		Gemara: GemaraConfig{
			Sources: []GemaraSourceEntry{
				{Source: "oci://ghcr.io/org/catalog:latest"},
			},
		},
		Schemas: []SchemaRef{
			{Platform: "kubernetes-deployment"},
			{Platform: "ci-github-actions"},
		},
	}

	data, err := MarshalConfigYAML(cfg, "1.0.0", "2026-07-13")
	require.NoError(t, err)

	output := string(data)

	// Must start with the header comment
	assert.True(t, strings.HasPrefix(output, "# yaml-language-server:"), "should have yaml-language-server directive")
	assert.Contains(t, output, "# Generated by complypack init 1.0.0 on 2026-07-13\n")

	// Must contain expected fields
	assert.Contains(t, output, "id: io.example.my-pack")
	assert.Contains(t, output, "evaluator-id: opa")
	assert.Contains(t, output, "version: 0.1.0")
	assert.Contains(t, output, "source: oci://ghcr.io/org/catalog:latest")
	assert.Contains(t, output, "platform: kubernetes-deployment")
	assert.Contains(t, output, "platform: ci-github-actions")

	// Must use 2-space indentation (not 4-space)
	assert.Contains(t, output, "\n  sources:")
	assert.NotContains(t, output, "    sources:")

	// Must not contain empty optional sections
	assert.NotContains(t, output, "policies:")
	assert.NotContains(t, output, "tests:")
	assert.NotContains(t, output, "fixtures:")
	assert.NotContains(t, output, "output:")

	// Must be valid YAML that round-trips
	var parsed ComplyPackConfig
	require.NoError(t, yaml.Unmarshal(data, &parsed))
	assert.Equal(t, cfg.ID, parsed.ID)
	assert.Equal(t, cfg.EvaluatorID, parsed.EvaluatorID)
	assert.Equal(t, cfg.Version, parsed.Version)
	assert.Equal(t, len(cfg.Schemas), len(parsed.Schemas))
}

func TestConfigHeader(t *testing.T) {
	t.Run("includes schema directive, version, and date", func(t *testing.T) {
		header := ConfigHeader("1.2.3", "2026-07-13")
		assert.Contains(t, header, "# yaml-language-server: $schema=")
		assert.Contains(t, header, "# Generated by complypack init 1.2.3 on 2026-07-13")
		assert.True(t, strings.HasSuffix(header, "\n"), "header should end with newline")
	})

	t.Run("includes devel version", func(t *testing.T) {
		header := ConfigHeader("(devel)", "2026-01-01")
		assert.Contains(t, header, "# Generated by complypack init (devel) on 2026-01-01")
	})
}

func TestWriteConfigFile(t *testing.T) {
	content := []byte("id: test\n")

	t.Run("writes new file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "complypack.yaml")

		err := WriteConfigFile(path, content, false, false)
		require.NoError(t, err)

		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, content, got)

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
	})

	t.Run("refuses to overwrite without force", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "complypack.yaml")
		require.NoError(t, os.WriteFile(path, []byte("existing"), 0600))

		err := WriteConfigFile(path, content, false, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")

		// Original content preserved
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, []byte("existing"), got)
	})

	t.Run("overwrites with force", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "complypack.yaml")
		require.NoError(t, os.WriteFile(path, []byte("existing"), 0600))

		err := WriteConfigFile(path, content, true, false)
		require.NoError(t, err)

		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})

	t.Run("refuses symlink without force", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target.yaml")
		link := filepath.Join(dir, "complypack.yaml")
		require.NoError(t, os.WriteFile(target, []byte("target"), 0600))
		require.NoError(t, os.Symlink(target, link))

		err := WriteConfigFile(link, content, false, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "symbolic link")
	})

	t.Run("overwrites symlink with force", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target.yaml")
		link := filepath.Join(dir, "complypack.yaml")
		require.NoError(t, os.WriteFile(target, []byte("target"), 0600))
		require.NoError(t, os.Symlink(target, link))

		err := WriteConfigFile(link, content, true, false)
		require.NoError(t, err)

		got, err := os.ReadFile(link)
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})

	t.Run("fails when parent dir missing without parents flag", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "sub", "deep", "complypack.yaml")

		err := WriteConfigFile(path, content, false, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
		assert.Contains(t, err.Error(), "--parents")
	})

	t.Run("creates parent dirs with parents flag", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "sub", "deep", "complypack.yaml")

		err := WriteConfigFile(path, content, false, true)
		require.NoError(t, err)

		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, content, got)

		// Verify parent dir permissions
		info, err := os.Stat(filepath.Join(dir, "sub"))
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})
}

func TestCheckCredentialURIs(t *testing.T) {
	t.Run("no credentials", func(t *testing.T) {
		var buf bytes.Buffer
		err := CheckCredentialURIs(&buf, []string{"oci://ghcr.io/org/catalog:v1"}, false)
		assert.NoError(t, err)
		assert.Empty(t, buf.String())
	})

	t.Run("credentials rejected by default", func(t *testing.T) {
		var buf bytes.Buffer
		err := CheckCredentialURIs(&buf, []string{"https://token:secret@registry.example.com/catalog"}, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "embedded credentials")
		assert.Contains(t, err.Error(), "--allow-credentials")
	})

	t.Run("user only rejected by default", func(t *testing.T) {
		var buf bytes.Buffer
		err := CheckCredentialURIs(&buf, []string{"https://user@registry.example.com/catalog"}, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "embedded credentials")
	})

	t.Run("credentials allowed with flag", func(t *testing.T) {
		var buf bytes.Buffer
		err := CheckCredentialURIs(&buf, []string{"https://token:secret@registry.example.com/catalog"}, true)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "WARNING")
		assert.Contains(t, buf.String(), "credentials")
	})

	t.Run("nil sources", func(t *testing.T) {
		var buf bytes.Buffer
		err := CheckCredentialURIs(&buf, nil, false)
		assert.NoError(t, err)
		assert.Empty(t, buf.String())
	})
}

func TestLoadConfig_WarningsWrittenToWriter(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	// Write a config with a typo key
	content := []byte("version: 0.1.0\nevalutaor-id: opa\nschemas:\n  - platform: kubernetes\ngemara:\n  source: catalogs/controls.yaml\n")
	require.NoError(t, os.WriteFile(configPath, content, 0600))

	var buf bytes.Buffer
	cfg, err := LoadConfig(configPath, false, &buf)
	require.NoError(t, err)
	assert.Equal(t, "0.1.0", cfg.Version)
	assert.Empty(t, cfg.EvaluatorID) // misspelled, so not parsed
	assert.Contains(t, buf.String(), "WARNING")
	assert.Contains(t, buf.String(), "evalutaor-id")
}

func TestLoadConfig_GemaraBothSourceAndSources(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	configContent := `version: 0.1.0
gemara:
  source: catalogs/controls.yaml
  sources:
    - source: catalogs/other.yaml
schemas:
  - platform: kubernetes
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	_, err = LoadConfig(configPath, false, os.Stderr)
	assert.Error(t, err)
	// Schema validation now catches this via oneOf constraint before UnmarshalYAML
	assert.Contains(t, err.Error(), "schema validation")
}

func TestLoadConfig_StrictRejectsUnknownKeys(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complypack.yaml")

	content := []byte("version: 0.1.0\nevalutaor-id: opa\n")
	require.NoError(t, os.WriteFile(configPath, content, 0600))

	_, err := LoadConfig(configPath, true, os.Stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "strict mode")
}

func TestBuildConfig(t *testing.T) {
	cfg := BuildConfig(
		"io.example.test",
		"opa",
		"1.0.0",
		[]GemaraSourceEntry{{Source: "oci://ghcr.io/org/catalog:v1"}},
		[]SchemaRef{{Platform: "kubernetes-deployment"}},
	)

	assert.Equal(t, "io.example.test", cfg.ID)
	assert.Equal(t, "opa", cfg.EvaluatorID)
	assert.Equal(t, "1.0.0", cfg.Version)
	require.Len(t, cfg.Gemara.Sources, 1)
	assert.Equal(t, "oci://ghcr.io/org/catalog:v1", cfg.Gemara.Sources[0].Source)
	require.Len(t, cfg.Schemas, 1)
	assert.Equal(t, "kubernetes-deployment", cfg.Schemas[0].Platform)
}

func TestBuildConfig_EmptyOptionalFields(t *testing.T) {
	cfg := BuildConfig("", "", "", nil, nil)

	assert.Empty(t, cfg.ID)
	assert.Empty(t, cfg.EvaluatorID)
	assert.Empty(t, cfg.Version)
	assert.Nil(t, cfg.Gemara.Sources)
	assert.Nil(t, cfg.Schemas)
}
