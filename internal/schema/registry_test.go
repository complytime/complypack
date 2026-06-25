// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_Load_Dispatch(t *testing.T) {
	ctx := context.Background()

	t.Run("empty registry returns error", func(t *testing.T) {
		reg := NewRegistry()
		_, err := reg.Load(ctx, "file:///some/path", "test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no loader matched")
	})

	t.Run("legacy loader catches unrecognized schemes", func(t *testing.T) {
		reg := DefaultRegistry()
		_, err := reg.Load(ctx, "ftp://host/file", "test")
		assert.Error(t, err) // LegacyLoader matches but file doesn't exist
	})
}

func TestRegistry_FileLoader(t *testing.T) {
	ctx := context.Background()
	reg := DefaultRegistry()

	// Create a temp JSON schema file
	tmpDir := t.TempDir()
	path := tmpDir + "/test.json"
	err := writeTestFile(path, `{"type": "object"}`)
	require.NoError(t, err)

	schema, err := reg.Load(ctx, "file://"+path, "test-platform")
	require.NoError(t, err)
	assert.Equal(t, "test-platform", schema.Platform)
	assert.Contains(t, string(schema.Bytes), "object")
}

func TestRegistry_LegacyLoader(t *testing.T) {
	ctx := context.Background()
	reg := DefaultRegistry()

	tmpDir := t.TempDir()
	path := tmpDir + "/test.json"
	err := writeTestFile(path, `{"type": "object"}`)
	require.NoError(t, err)

	schema, err := reg.Load(ctx, path, "test-platform")
	require.NoError(t, err)
	assert.Equal(t, "test-platform", schema.Platform)
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}
