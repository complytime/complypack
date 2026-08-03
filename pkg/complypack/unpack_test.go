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
	if len(result.Config.Source) != len(cfg.Source) {
		t.Fatalf("Source length = %d, want %d", len(result.Config.Source), len(cfg.Source))
	}
	if result.Config.Source[0].PolicyID != cfg.Source[0].PolicyID {
		t.Errorf("PolicyID = %q, want %q",
			result.Config.Source[0].PolicyID, cfg.Source[0].PolicyID)
	}
	if len(result.Config.Source[0].GemaraContent) != len(cfg.Source[0].GemaraContent) {
		t.Fatalf("GemaraContent length = %d, want %d",
			len(result.Config.Source[0].GemaraContent), len(cfg.Source[0].GemaraContent))
	}
	gotRef := result.Config.Source[0].GemaraContent[0]
	wantRef := cfg.Source[0].GemaraContent[0]
	if gotRef.ReferenceID != wantRef.ReferenceID {
		t.Errorf("GemaraContent ReferenceID = %q, want %q", gotRef.ReferenceID, wantRef.ReferenceID)
	}
	if gotRef.URI != wantRef.URI {
		t.Errorf("GemaraContent URI = %q, want %q", gotRef.URI, wantRef.URI)
	}
	if gotRef.Version != wantRef.Version {
		t.Errorf("GemaraContent Version = %q, want %q", gotRef.Version, wantRef.Version)
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
		ID:          "io.test.pack",
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
