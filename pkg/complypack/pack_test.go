// SPDX-License-Identifier: Apache-2.0

package complypack_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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
	if err != nil {
		t.Fatalf("Pack() error = %v", err)
	}

	// Verify descriptor returned
	if desc.Digest == "" {
		t.Error("descriptor has empty digest")
	}
	if desc.Size == 0 {
		t.Error("descriptor has zero size")
	}
	if desc.MediaType != ocispec.MediaTypeImageManifest {
		t.Errorf("descriptor MediaType = %q, want %q", desc.MediaType, ocispec.MediaTypeImageManifest)
	}
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
	if err != nil {
		t.Fatalf("Pack() error = %v", err)
	}

	if desc.Digest == "" {
		t.Error("descriptor has empty digest")
	}
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
	if err != nil {
		t.Fatalf("Pack() error = %v", err)
	}

	if desc.Digest == "" {
		t.Error("descriptor has empty digest")
	}
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
		if err == nil {
			t.Error("expected error for empty evaluator-id")
		}
		if !strings.Contains(err.Error(), "evaluator-id") {
			t.Errorf("error should mention evaluator-id, got: %v", err)
		}
	})

	t.Run("invalid config - empty version", func(t *testing.T) {
		cfg := complypack.Config{
			EvaluatorID: "io.complytime.opa",
		}
		content := strings.NewReader("content")

		_, err := complypack.Pack(ctx, store, cfg, content)
		if err == nil {
			t.Error("expected error for empty version")
		}
		if !strings.Contains(err.Error(), "version") {
			t.Errorf("error should mention version, got: %v", err)
		}
	})

	t.Run("empty content", func(t *testing.T) {
		cfg := complypack.Config{
			EvaluatorID: "io.complytime.opa",
			Version:     "1.0.0",
		}
		content := bytes.NewReader([]byte{})

		_, err := complypack.Pack(ctx, store, cfg, content)
		if err == nil {
			t.Error("expected error for empty content")
		}
	})
}
