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
		cmd, _, err := root.Find([]string{"validate"})
		require.NoError(t, err)
		assert.Equal(t, "validate", cmd.Name())
		assert.NotEmpty(t, cmd.Short, "validate command should have a short description")
	})

	t.Run("has flags", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"validate"})
		require.NoError(t, err)

		flags := cmd.Flags()
		assert.NotNil(t, flags.Lookup("strict"), "should have --strict flag")
	})

	t.Run("flag defaults", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"validate"})
		require.NoError(t, err)

		flags := cmd.Flags()
		assert.Equal(t, "false", flags.Lookup("strict").DefValue)
	})
}

// validConfigYAML is a minimal valid complypack.yaml for testing.
const validConfigYAML = `version: 0.1.0
schemas:
  - platform: kubernetes-deployment
`

// invalidConfigYAML has a version that violates the semver pattern.
const invalidConfigYAML = `version: nope
schemas:
  - platform: kubernetes-deployment
`

// unknownFieldConfigYAML has a valid config plus an unknown field.
const unknownFieldConfigYAML = `version: 0.1.0
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
	path := writeTestConfig(t, dir, validConfigYAML)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"validate", path})

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
	root.SetArgs([]string{"validate", path})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestValidateEndToEnd_FileNotFound(t *testing.T) {
	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"validate", "nonexistent.yaml"})

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
	root.SetArgs([]string{"validate", path})

	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "is valid")
	assert.Contains(t, stderr.String(), "WARNING")
}

func TestValidateEndToEnd_UnknownFieldsStrict(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, unknownFieldConfigYAML)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"validate", "--strict", path})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestValidateEndToEnd_DefaultPath(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, validConfigYAML)

	// Change to the temp directory so default path resolves
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"validate"})

	err = root.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "is valid")
}
