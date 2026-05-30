// SPDX-License-Identifier: Apache-2.0

package complypack_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content/memory"

	"github.com/complytime/complypack/pkg/complypack"
)

func TestPackMinimal(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	cfg := complypack.Config{
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
		EvaluatorID: "io.complytime.opa",
		Version:     "1.0.0",
		Source: &complypack.Provenance{
			GemaraContent: "oci://registry/gemara/controls:v1",
			PolicyID:      "pol-123",
		},
	}

	content := strings.NewReader("fake policy content")

	desc, err := complypack.Pack(ctx, store, cfg, content)
	require.NoError(t, err)
	assert.NotEmpty(t, desc.Digest)
}

func TestPackWithAnnotations(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	cfg := complypack.Config{
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

	t.Run("invalid config - empty evaluator-id", func(t *testing.T) {
		cfg := complypack.Config{
			Version: "1.0.0",
		}
		content := strings.NewReader("content")

		_, err := complypack.Pack(ctx, store, cfg, content)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "evaluator-id")
	})

	t.Run("invalid config - empty version", func(t *testing.T) {
		cfg := complypack.Config{
			EvaluatorID: "io.complytime.opa",
		}
		content := strings.NewReader("content")

		_, err := complypack.Pack(ctx, store, cfg, content)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "version")
	})

	t.Run("empty content", func(t *testing.T) {
		cfg := complypack.Config{
			EvaluatorID: "io.complytime.opa",
			Version:     "1.0.0",
		}
		content := bytes.NewReader([]byte{})

		_, err := complypack.Pack(ctx, store, cfg, content)
		assert.ErrorIs(t, err, complypack.ErrEmptyContent)
	})

	t.Run("content too large", func(t *testing.T) {
		cfg := complypack.Config{
			EvaluatorID: "io.complytime.opa",
			Version:     "1.0.0",
		}
		// Create content larger than MaxContentSize (100MB)
		largeContent := strings.NewReader(strings.Repeat("x", complypack.MaxContentSize+1))

		_, err := complypack.Pack(ctx, store, cfg, largeContent)
		assert.ErrorIs(t, err, complypack.ErrContentTooLarge)
	})
}
