// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"log"

	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/packer"
	"github.com/complytime/complypack/internal/registry"
	"github.com/complytime/complypack/pkg/complypack"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
)

func packCmd() *cobra.Command {
	var (
		configPath string
		plainHTTP  bool
	)

	cmd := &cobra.Command{
		Use:   "pack <content-dir> <oci-reference>",
		Short: "Pack policy content into a ComplyPack OCI artifact",
		Long: `Pack a directory of policy content into a ComplyPack OCI artifact
and push it to an OCI registry.

Reads evaluator-id, version, and gemara source from complypack.yaml.
The content directory is archived as a tar.gz and stored as the
artifact's opaque content layer.

Examples:
  complypack pack policy/ ghcr.io/org/my-policies:v1.0.0
  complypack pack policy/ localhost:5001/test:latest --plain-http`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			contentDir := args[0]
			ref := args[1]

			// Load config
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			if err := cfg.ValidateForPack(); err != nil {
				return fmt.Errorf("config validation: %w", err)
			}

			// Build complypack config from complypack.yaml
			packCfg := complypack.Config{
				EvaluatorID: cfg.EvaluatorID,
				Version:     cfg.Version,
			}

			// Create tarball from content directory
			log.Printf("Packing %s...", contentDir)
			content, err := packer.TarGzipDir(contentDir)
			if err != nil {
				return fmt.Errorf("creating archive: %w", err)
			}

			// Pack into OCI artifact
			store := memory.New()
			desc, err := complypack.Pack(ctx, store, packCfg, content)
			if err != nil {
				return fmt.Errorf("packing artifact: %w", err)
			}

			// Tag
			tag := registry.ParseTag(ref)
			if err := store.Tag(ctx, desc, tag); err != nil {
				return fmt.Errorf("tagging artifact: %w", err)
			}

			// Push to registry
			credFunc, err := registry.NewCredentialFunc()
			if err != nil {
				return fmt.Errorf("loading credentials: %w", err)
			}

			repo, err := registry.NewRepository(ref, credFunc, plainHTTP)
			if err != nil {
				return fmt.Errorf("creating repository: %w", err)
			}

			log.Printf("Pushing to %s...", ref)
			_, err = oras.Copy(ctx, store, tag, repo, tag, oras.DefaultCopyOptions)
			if err != nil {
				return fmt.Errorf("pushing artifact: %w", err)
			}

			log.Printf("Published %s", ref)
			log.Printf("  evaluator-id: %s", packCfg.EvaluatorID)
			log.Printf("  version:      %s", packCfg.Version)
			log.Printf("  digest:       %s", desc.Digest)

			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "complypack.yaml", "Path to complypack.yaml")
	cmd.Flags().BoolVar(&plainHTTP, "plain-http", false, "Use HTTP instead of HTTPS for the registry")

	return cmd
}
