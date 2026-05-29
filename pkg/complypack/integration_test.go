// SPDX-License-Identifier: Apache-2.0

package complypack_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/content/memory"

	"github.com/complytime/complypack/pkg/complypack"
)

func TestIntegrationMemoryStore(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	// Pack
	cfg := complypack.Config{
		EvaluatorID: "io.complytime.opa",
		Version:     "1.0.0",
		Source: &complypack.Provenance{
			GemaraContent: "oci://registry/gemara/controls:v1",
			PolicyID:      "test-policy-001",
		},
	}

	content := "This is test policy content that would normally be an OPA bundle"
	annotations := map[string]string{
		"org.opencontainers.image.authors": "test@example.com",
		"test.annotation":                  "integration-test",
	}

	desc, err := complypack.Pack(ctx, store, cfg, strings.NewReader(content),
		complypack.WithAnnotations(annotations))
	if err != nil {
		t.Fatalf("Pack() error = %v", err)
	}

	// Unpack
	result, err := complypack.Unpack(ctx, store, desc)
	if err != nil {
		t.Fatalf("Unpack() error = %v", err)
	}
	defer func() { _ = result.Content.Close() }()

	// Verify round-trip
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
		t.Errorf("GemaraContent mismatch")
	}
	if result.Config.Source.PolicyID != cfg.Source.PolicyID {
		t.Errorf("PolicyID mismatch")
	}

	unpackedContent, err := io.ReadAll(result.Content)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(unpackedContent) != content {
		t.Errorf("content = %q, want %q", string(unpackedContent), content)
	}
}

func TestIntegrationFileStore(t *testing.T) {
	ctx := context.Background()

	// Create temp directory for OCI layout
	tmpDir, err := os.MkdirTemp("", "complypack-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := file.New(tmpDir)
	if err != nil {
		t.Fatalf("file.New() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	// Pack
	cfg := complypack.Config{
		EvaluatorID: "io.complytime.cel",
		Version:     "2.0.0",
	}

	content := "CEL policy content example"
	desc, err := complypack.Pack(ctx, store, cfg, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Pack() error = %v", err)
	}

	// Unpack
	result, err := complypack.Unpack(ctx, store, desc)
	if err != nil {
		t.Fatalf("Unpack() error = %v", err)
	}
	defer func() { _ = result.Content.Close() }()

	// Verify
	if result.Config.EvaluatorID != cfg.EvaluatorID {
		t.Errorf("EvaluatorID = %q, want %q", result.Config.EvaluatorID, cfg.EvaluatorID)
	}

	unpackedContent, err := io.ReadAll(result.Content)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(unpackedContent) != content {
		t.Errorf("content mismatch")
	}
}

func TestIntegrationLargeContent(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	cfg := complypack.Config{
		EvaluatorID: "io.complytime.opa",
		Version:     "1.0.0",
	}

	// Create 1MB of content
	largeContent := strings.Repeat("x", 1024*1024)

	desc, err := complypack.Pack(ctx, store, cfg, strings.NewReader(largeContent))
	if err != nil {
		t.Fatalf("Pack() error = %v", err)
	}

	result, err := complypack.Unpack(ctx, store, desc)
	if err != nil {
		t.Fatalf("Unpack() error = %v", err)
	}
	defer func() { _ = result.Content.Close() }()

	unpackedContent, err := io.ReadAll(result.Content)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if len(unpackedContent) != len(largeContent) {
		t.Errorf("content length = %d, want %d", len(unpackedContent), len(largeContent))
	}
}
