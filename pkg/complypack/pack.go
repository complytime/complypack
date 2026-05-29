// SPDX-License-Identifier: Apache-2.0

package complypack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
)

// Pack assembles a ComplyPack OCI artifact from config and opaque content.
// The content is stored as a single layer with MediaTypeContent.
// The config is stored with MediaTypeConfig.
//
// Options:
//   - WithSigning(keyPath) enables keyed signing
//   - WithKeylessSigning(identity, issuer) enables OIDC-based keyless signing
//   - WithAnnotations(map) adds OCI manifest annotations
//
// Returns the OCI manifest descriptor pointing to the packed artifact.
func Pack(ctx context.Context, store content.Storage, cfg Config, content io.Reader, opts ...PackOption) (ocispec.Descriptor, error) {
	// Validate config
	if err := cfg.Validate(); err != nil {
		return ocispec.Descriptor{}, err
	}

	// Apply options
	options := &packOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Read content into memory (needed for digest calculation)
	contentBytes, err := io.ReadAll(content)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("reading content: %w", err)
	}
	if len(contentBytes) == 0 {
		return ocispec.Descriptor{}, ErrEmptyContent
	}

	// Push config blob
	configData, err := json.Marshal(cfg)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("marshaling config: %w", err)
	}
	configDesc, err := pushBlob(ctx, store, MediaTypeConfig, configData)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing config blob: %w", err)
	}

	// Push content blob
	contentDesc, err := pushBlob(ctx, store, MediaTypeContent, contentBytes)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing content blob: %w", err)
	}

	// Build manifest annotations
	annotations := make(map[string]string)
	if options.annotations != nil {
		for k, v := range options.annotations {
			annotations[k] = v
		}
	}
	// Add evaluator-id annotation for discoverability
	annotations["complypack.evaluator-id"] = cfg.EvaluatorID

	// Pack manifest
	manifestDesc, err := oras.PackManifest(ctx, store,
		oras.PackManifestVersion1_1,
		MediaTypeArtifact,
		oras.PackManifestOptions{
			ConfigDescriptor:    &configDesc,
			Layers:              []ocispec.Descriptor{contentDesc},
			ManifestAnnotations: annotations,
		})
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("packing manifest: %w", err)
	}

	// Sign if requested
	if err := sign(ctx, store, manifestDesc, options); err != nil {
		return ocispec.Descriptor{}, err
	}

	return manifestDesc, nil
}

// pushBlob pushes a blob to the store and returns its descriptor.
// Ignores ErrAlreadyExists since content-addressable storage is idempotent.
func pushBlob(ctx context.Context, store content.Storage, mediaType string, data []byte) (ocispec.Descriptor, error) {
	desc := ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(data),
		Size:      int64(len(data)),
	}

	err := store.Push(ctx, desc, bytes.NewReader(data))
	if err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return ocispec.Descriptor{}, err
	}

	return desc, nil
}
