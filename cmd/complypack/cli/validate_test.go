// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCommand(t *testing.T) {
	root := New()

	t.Run("command exists", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"config", "validate"})
		require.NoError(t, err)
		assert.Equal(t, "validate", cmd.Name())
		assert.NotEmpty(t, cmd.Short, "validate command should have a short description")
	})

	t.Run("has flags", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"config", "validate"})
		require.NoError(t, err)

		flags := cmd.Flags()
		assert.NotNil(t, flags.Lookup("unknown-fields"), "should have --unknown-fields flag")
		assert.NotNil(t, flags.Lookup("scope"), "should have --scope flag")
	})

	t.Run("flag defaults", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"config", "validate"})
		require.NoError(t, err)

		flags := cmd.Flags()
		assert.Equal(t, "warn", flags.Lookup("unknown-fields").DefValue)
		assert.Equal(t, "[]", flags.Lookup("scope").DefValue)
	})
}

// validConfigYAML is a minimal valid complypack.yaml for testing.
const validConfigYAML = `version: 0.1.0
schemas:
  - platform: kubernetes-deployment
`

// fullConfigYAML has all fields needed for every scope.
const fullConfigYAML = `id: io.test.example
evaluator-id: opa
version: 0.1.0
gemara:
  sources:
    - source: catalogs/controls.yaml
schemas:
  - platform: kubernetes-deployment
`

// invalidConfigYAML has a version that violates the semver pattern.
const invalidConfigYAML = `version: nope
schemas:
  - platform: kubernetes-deployment
`

// unknownFieldConfigYAML has a full valid config plus an unknown field.
const unknownFieldConfigYAML = `id: io.test.example
evaluator-id: opa
version: 0.1.0
gemara:
  sources:
    - source: catalogs/controls.yaml
schemas:
  - platform: kubernetes-deployment
bogus-field: unexpected
`

func writeTestConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "complypack.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func TestValidateEndToEnd_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, fullConfigYAML)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"config", "validate", path})

	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "is valid")
}

func TestValidateEndToEnd_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, invalidConfigYAML)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"config", "validate", path})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestValidateEndToEnd_FileNotFound(t *testing.T) {
	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"config", "validate", "nonexistent.yaml"})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestValidateEndToEnd_UnknownFieldsLenient(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, unknownFieldConfigYAML)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"config", "validate", path})

	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "is valid")
	assert.Contains(t, stderr.String(), "WARNING")
}

func TestValidateEndToEnd_UnknownFieldsError(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, unknownFieldConfigYAML)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"config", "validate", "--unknown-fields=error", path})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestValidateEndToEnd_DefaultPath(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, fullConfigYAML)

	t.Chdir(dir)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"config", "validate"})

	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "is valid")
}

func TestValidateEndToEnd_ScopePack(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, fullConfigYAML)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"config", "validate", "--scope", "pack", path})

	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "pack: ok")
	assert.Contains(t, stdout.String(), "is valid")
}

func TestValidateEndToEnd_ScopePackMissingFields(t *testing.T) {
	dir := t.TempDir()
	// validConfigYAML lacks id and evaluator-id, which pack scope requires
	path := writeTestConfig(t, dir, validConfigYAML)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"config", "validate", "--scope", "pack", path})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, stdout.String(), "pack: FAIL")
}

func TestValidateEndToEnd_InvalidScope(t *testing.T) {
	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"config", "validate", "--scope", "bogus", "nonexistent.yaml"})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --scope values")
}

func TestValidateEndToEnd_BothFlagsInvalid(t *testing.T) {
	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"config", "validate", "--scope", "bogus", "--unknown-fields", "bogus", "nonexistent.yaml"})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --scope values")
	assert.Contains(t, err.Error(), "invalid --unknown-fields value")
}

func TestValidateEndToEnd_InvalidUnknownFields(t *testing.T) {
	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"config", "validate", "--unknown-fields", "bogus", "nonexistent.yaml"})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --unknown-fields value")
}
