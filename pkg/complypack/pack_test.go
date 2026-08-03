// SPDX-License-Identifier: Apache-2.0

package complypack_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"

	"github.com/complytime/complypack/pkg/complypack"
)

func TestPackMinimal(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	cfg := complypack.Config{
		ID:          "io.test.pack",
		EvaluatorID: "io.complytime.opa",
		Version:     "1.0.0",
	}

	content := strings.NewReader("fake policy content")

	desc, err := complypack.Pack(ctx, store, cfg, content)
	require.NoError(t, err)

	// Verify descriptor returned
	assert.NotEmpty(t, desc.Digest, "descriptor should have a digest")
	assert.NotZero(t, desc.Size, "descriptor should have a size")
	assert.Equal(t, ocispec.MediaTypeImageManifest, desc.MediaType)
}

func TestPackWithProvenance(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	cfg := complypack.Config{
		ID:          "io.test.pack",
		EvaluatorID: "io.complytime.opa",
		Version:     "1.0.0",
		Source: []complypack.Provenance{
			{
				PolicyID: "pol-123",
				GemaraContent: []complypack.GemaraRef{
					{
						ReferenceID: "ref-1",
						URI:         "oci://registry/gemara/controls:v1",
						Version:     "1.0.0",
					},
				},
			},
		},
	}

	content := strings.NewReader("fake policy content")

	desc, err := complypack.Pack(ctx, store, cfg, content)
	require.NoError(t, err)
	assert.NotEmpty(t, desc.Digest)

	// The config blob must carry the provenance as a "source" array.
	raw := fetchConfigBlob(t, ctx, store, desc)
	source, ok := raw["source"].([]any)
	require.True(t, ok, "config blob should contain a source array")
	require.Len(t, source, 1)
	entry, ok := source[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "pol-123", entry["policy-id"])

	gemaraContent, ok := entry["gemara-content"].([]any)
	require.True(t, ok, "source entry should contain a gemara-content array")
	require.Len(t, gemaraContent, 1)
	ref, ok := gemaraContent[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ref-1", ref["reference-id"])
	assert.Equal(t, "oci://registry/gemara/controls:v1", ref["uri"])
	assert.Equal(t, "1.0.0", ref["version"])
}

func TestPackWithoutProvenanceOmitsSource(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	cfg := complypack.Config{
		ID:          "io.test.pack",
		EvaluatorID: "io.complytime.opa",
		Version:     "1.0.0",
	}

	desc, err := complypack.Pack(ctx, store, cfg, strings.NewReader("fake policy content"))
	require.NoError(t, err)

	raw := fetchConfigBlob(t, ctx, store, desc)
	_, exists := raw["source"]
	assert.False(t, exists, "config blob should omit source when no provenance is set")
}

func TestPackProvenanceByteIdentical(t *testing.T) {
	ctx := context.Background()

	cfg := complypack.Config{
		ID:          "io.test.pack",
		EvaluatorID: "io.complytime.opa",
		Version:     "1.0.0",
		Source: []complypack.Provenance{
			{
				PolicyID: "pol-123",
				GemaraContent: []complypack.GemaraRef{
					{ReferenceID: "ref-1", URI: "oci://registry/gemara/controls:v1", Version: "1.0.0"},
				},
			},
		},
	}

	store1 := memory.New()
	desc1, err := complypack.Pack(ctx, store1, cfg, strings.NewReader("fake policy content"))
	require.NoError(t, err)

	store2 := memory.New()
	desc2, err := complypack.Pack(ctx, store2, cfg, strings.NewReader("fake policy content"))
	require.NoError(t, err)

	assert.Equal(t, desc1.Digest, desc2.Digest,
		"identical input must produce a byte-identical config blob and manifest digest")
}

// fetchConfigBlob reads the manifest referenced by desc and returns its config
// blob decoded into a generic JSON map.
func fetchConfigBlob(t *testing.T, ctx context.Context, store content.Fetcher, desc ocispec.Descriptor) map[string]any {
	t.Helper()

	manifestBytes, err := content.FetchAll(ctx, store, desc)
	require.NoError(t, err)

	var manifest ocispec.Manifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))

	configBytes, err := content.FetchAll(ctx, store, manifest.Config)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(configBytes, &raw))
	return raw
}

func TestPackWithAnnotations(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	cfg := complypack.Config{
		ID:          "io.test.pack",
		EvaluatorID: "io.complytime.opa",
		Version:     "1.0.0",
	}

	content := strings.NewReader("fake policy content")

	annotations := map[string]string{
		"org.opencontainers.image.authors": "test@example.com",
		"custom.annotation":                "value",
	}

	desc, err := complypack.Pack(ctx, store, cfg, content, complypack.WithAnnotations(annotations))
	require.NoError(t, err)
	assert.NotEmpty(t, desc.Digest)
}

func TestPackErrors(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	t.Run("invalid config - missing id", func(t *testing.T) {
		cfg := complypack.Config{
			EvaluatorID: "opa",
			Version:     "1.0.0",
		}
		content := strings.NewReader("content")

		_, err := complypack.Pack(ctx, store, cfg, content)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "id is required")
	})

	t.Run("invalid config - empty evaluator-id", func(t *testing.T) {
		cfg := complypack.Config{
			ID:      "io.test.pack",
			Version: "1.0.0",
		}
		content := strings.NewReader("content")

		_, err := complypack.Pack(ctx, store, cfg, content)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "evaluator-id")
	})

	t.Run("invalid config - empty version", func(t *testing.T) {
		cfg := complypack.Config{
			ID:          "io.test.pack",
			EvaluatorID: "opa",
		}
		content := strings.NewReader("content")

		_, err := complypack.Pack(ctx, store, cfg, content)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "version")
	})

	t.Run("empty content", func(t *testing.T) {
		cfg := complypack.Config{
			ID:          "io.test.pack",
			EvaluatorID: "io.complytime.opa",
			Version:     "1.0.0",
		}
		content := bytes.NewReader([]byte{})

		_, err := complypack.Pack(ctx, store, cfg, content)
		assert.ErrorIs(t, err, complypack.ErrEmptyContent)
	})

	t.Run("content too large", func(t *testing.T) {
		cfg := complypack.Config{
			ID:          "io.test.pack",
			EvaluatorID: "io.complytime.opa",
			Version:     "1.0.0",
		}
		// Create content larger than MaxContentSize (100MB)
		largeContent := strings.NewReader(strings.Repeat("x", complypack.MaxContentSize+1))

		_, err := complypack.Pack(ctx, store, cfg, largeContent)
		assert.ErrorIs(t, err, complypack.ErrContentTooLarge)
	})
}
