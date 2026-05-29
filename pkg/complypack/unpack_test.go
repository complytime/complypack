// SPDX-License-Identifier: Apache-2.0

package complypack_test

import (
	"context"
	"io"
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/memory"

	"github.com/complytime/complypack/pkg/complypack"
)

func TestUnpackRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	// Pack
	cfg := complypack.Config{
		EvaluatorID: "io.complytime.opa",
		Version:     "1.0.0",
		Source: &complypack.Provenance{
			GemaraContent: "oci://registry/gemara/controls:v1",
			PolicyID:      "pol-123",
		},
	}

	originalContent := "fake policy content"
	packDesc, err := complypack.Pack(ctx, store, cfg, strings.NewReader(originalContent))
	if err != nil {
		t.Fatalf("Pack() error = %v", err)
	}

	// Unpack
	result, err := complypack.Unpack(ctx, store, packDesc)
	if err != nil {
		t.Fatalf("Unpack() error = %v", err)
	}
	defer func() { _ = result.Content.Close() }()

	// Verify config
	if result.Config.EvaluatorID != cfg.EvaluatorID {
		t.Errorf("EvaluatorID = %q, want %q", result.Config.EvaluatorID, cfg.EvaluatorID)
	}
	if result.Config.Version != cfg.Version {
		t.Errorf("Version = %q, want %q", result.Config.Version, cfg.Version)
	}
	if result.Config.Source == nil {
		t.Fatal("Source is nil")
	}
	if result.Config.Source.GemaraContent != cfg.Source.GemaraContent {
		t.Errorf("GemaraContent = %q, want %q", result.Config.Source.GemaraContent, cfg.Source.GemaraContent)
	}

	// Verify content
	unpackedContent, err := io.ReadAll(result.Content)
	if err != nil {
		t.Fatalf("ReadAll(content) error = %v", err)
	}
	if string(unpackedContent) != originalContent {
		t.Errorf("content = %q, want %q", string(unpackedContent), originalContent)
	}
}

func TestUnpackMinimal(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	// Pack minimal config
	cfg := complypack.Config{
		EvaluatorID: "io.complytime.opa",
		Version:     "1.0.0",
	}

	packDesc, err := complypack.Pack(ctx, store, cfg, strings.NewReader("content"))
	if err != nil {
		t.Fatalf("Pack() error = %v", err)
	}

	// Unpack
	result, err := complypack.Unpack(ctx, store, packDesc)
	if err != nil {
		t.Fatalf("Unpack() error = %v", err)
	}
	defer func() { _ = result.Content.Close() }()

	// Verify no provenance
	if result.Config.Source != nil {
		t.Error("Source should be nil for minimal config")
	}
}

func TestUnpackErrors(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	t.Run("descriptor not found", func(t *testing.T) {
		// Create a descriptor that doesn't exist in store
		fakeDesc := ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageManifest,
			Digest:    "sha256:0000000000000000000000000000000000000000000000000000000000000000",
			Size:      100,
		}

		_, err := complypack.Unpack(ctx, store, fakeDesc)
		if err == nil {
			t.Error("expected error for non-existent descriptor")
		}
	})
}
