// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/complytime/complypack/internal/registry"
	"github.com/complytime/complypack/pkg/complypack"
	"github.com/gemaraproj/go-gemara/bundle"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
)

type catalogPullOptions struct {
	ref       string
	output    string
	plainHTTP bool
}

// catalogPullCmd creates the `catalog pull` command.
func catalogPullCmd() *cobra.Command {
	opts := &catalogPullOptions{}

	cmd := &cobra.Command{
		Use:   "pull <reference>",
		Short: "Pull a Gemara control catalog from an OCI registry",
		Long: `Pull a Gemara control catalog from an OCI registry and output it to stdout or a file.

The reference should be a full OCI reference like:
  ghcr.io/org/controls:v1.0
  localhost:5000/catalogs/cis-benchmark@sha256:abc...

Authentication uses the Docker credential chain (same as docker login).`,
		Example: `  # Pull and output to stdout
  complypack catalog pull ghcr.io/org/controls:v1.0

  # Save to a file
  complypack catalog pull ghcr.io/org/controls:v1.0 --output controls.yaml

  # Pull from a local registry
  complypack catalog pull http://localhost:5000/controls:latest --plain-http`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ref = args[0]
			return opts.run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Write catalog to file instead of stdout")
	cmd.Flags().BoolVar(&opts.plainHTTP, "plain-http", false, "Use HTTP instead of HTTPS")

	return cmd
}

func (o *catalogPullOptions) run(ctx context.Context) error {
	if o.ref == "" {
		return fmt.Errorf("reference cannot be empty")
	}

	// Validate output path if provided
	if o.output != "" {
		if err := validateOutputPath(o.output); err != nil {
			return err
		}
	}

	// Get Docker credentials
	credFunc, err := registry.NewCredentialFunc()
	if err != nil {
		return fmt.Errorf("failed to load Docker credentials: %w", err)
	}

	// Create remote repository
	repo, err := registry.NewRepository(o.ref, credFunc, o.plainHTTP)
	if err != nil {
		return err
	}

	// Extract tag from reference
	tag := registry.ParseTag(o.ref)

	// Resolve manifest descriptor to check size before downloading
	// Use %q to sanitize reference output (prevents ANSI escape sequences)
	fmt.Fprintf(os.Stderr, "Resolving %q...\n", o.ref)
	manifestDesc, err := repo.Resolve(ctx, tag)
	if err != nil {
		return fmt.Errorf("failed to resolve reference: %w", err)
	}

	// Check manifest size to prevent memory exhaustion before downloading
	if err := validateArtifactSize(manifestDesc); err != nil {
		return err
	}

	// Copy from remote to memory store
	fmt.Fprintf(os.Stderr, "Pulling artifact...\n")
	store := memory.New()
	_, err = oras.Copy(ctx, repo, tag, store, tag, oras.DefaultCopyOptions)
	if err != nil {
		return fmt.Errorf("failed to pull from registry: %w", err)
	}

	// Unpack the Gemara bundle
	b, err := bundle.Unpack(ctx, store, tag)
	if err != nil {
		return fmt.Errorf("failed to unpack bundle: %w", err)
	}

	// Report imports if present
	if len(b.Imports) > 0 {
		fmt.Fprintf(os.Stderr, "Bundle contains %d import(s)\n", len(b.Imports))
	}

	// Get the primary catalog file (first in Files array)
	if len(b.Files) == 0 {
		return fmt.Errorf("bundle contains no files")
	}
	catalogData := b.Files[0].Data

	// Write catalog to output
	writer := os.Stdout
	var outputFile *os.File
	if o.output != "" {
		// Use os.OpenFile with explicit permissions and flags
		// O_TRUNC: truncate if exists, O_CREATE: create if not exists, O_WRONLY: write-only
		f, err := os.OpenFile(o.output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close() // Best-effort close on early return
		outputFile = f
		writer = f
		fmt.Fprintf(os.Stderr, "Writing catalog to %q\n", o.output)
	}

	if _, err := writer.Write(catalogData); err != nil {
		return fmt.Errorf("failed to write catalog: %w", err)
	}

	// Explicitly check close error for output file to catch write errors
	// on buffered/networked filesystems
	if outputFile != nil {
		if err := outputFile.Close(); err != nil {
			return fmt.Errorf("failed to close output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Successfully pulled catalog\n")
	}

	return nil
}

// validateOutputPath validates the output file path for security.
// Prevents path traversal and ensures safe file creation.
func validateOutputPath(path string) error {
	// Clean the path to normalize it
	cleanPath := filepath.Clean(path)

	// Reject absolute paths for safety
	if filepath.IsAbs(cleanPath) {
		return fmt.Errorf("absolute paths not allowed in --output flag: %q", path)
	}

	// Check for path traversal patterns
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path traversal not allowed in --output flag: %q", path)
	}

	// Check if path is a symlink (would follow symlink on write)
	if info, err := os.Lstat(cleanPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("output path is a symlink, refusing to write: %q", path)
		}
	}

	return nil
}

// validateArtifactSize checks if the artifact size is within acceptable limits.
// Uses the same limit as complypack.MaxContentSize to prevent memory exhaustion.
func validateArtifactSize(desc ocispec.Descriptor) error {
	if desc.Size > complypack.MaxContentSize {
		return fmt.Errorf("artifact size %d bytes exceeds maximum allowed size of %d bytes",
			desc.Size, complypack.MaxContentSize)
	}
	return nil
}
