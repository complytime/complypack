// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/complytime/complypack/internal/cache"
	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/source"
	"github.com/spf13/cobra"
)

// pullCmd creates the "pull" command.
func pullCmd() *cobra.Command {
	var (
		configPath string
		cacheDir   string
	)

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pre-warm the OCI cache for all configured sources",
		Long: `Pull all configured Gemara OCI sources into the local cache.

This command reads complypack.yaml, iterates all gemara sources,
and pulls each OCI source into the persistent cache directory.
File sources (file://) are skipped since they are already local.

The cache directory is resolved in order:
  1. --cache-dir flag value
  2. $XDG_CACHE_HOME/complypack
  3. $HOME/.cache/complypack

Examples:
  complypack pull
  complypack pull --config complypack.yaml
  complypack pull --cache-dir /tmp/my-cache`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedCacheDir, err := cache.ResolveDir(cacheDir)
			if err != nil {
				return fmt.Errorf("failed to resolve cache directory: %w", err)
			}

			cfg, err := config.LoadConfig(configPath, true, os.Stderr)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			result, err := pullSources(cmd.Context(), cfg, resolvedCacheDir)
			if err != nil {
				return err
			}

			log.Printf("Done: %d pulled, %d skipped", result.Pulled, result.Skipped)
			if len(result.Errors) > 0 {
				log.Printf("Errors:")
				for _, e := range result.Errors {
					log.Printf("  %s", e)
				}
				return fmt.Errorf("%d source(s) failed to pull", len(result.Errors))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "complypack.yaml", "Path to complypack.yaml config file")
	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", cache.CacheDirHelp)

	return cmd
}

// PullResult holds the outcome of pulling configured sources.
type PullResult struct {
	Pulled  int
	Skipped int
	Errors  []string
}

// pullSources iterates configured Gemara sources, pulls OCI sources into
// cacheDir, and skips file-based sources. This is the domain logic extracted
// from the cobra command for testability.
func pullSources(ctx context.Context, cfg *config.ComplyPackConfig, cacheDir string) (*PullResult, error) {
	if len(cfg.Gemara.Sources) == 0 {
		return nil, fmt.Errorf("no gemara sources configured")
	}

	log.Printf("Cache directory: %s", cacheDir)

	result := &PullResult{}

	for _, entry := range cfg.Gemara.Sources {
		src := entry.Source

		// Skip file sources — they are already local
		if strings.HasPrefix(src, "file://") || !source.IsOCIReference(src) {
			log.Printf("  Skipping %s (local source)", src)
			result.Skipped++
			continue
		}

		log.Printf("  Pulling %s...", src)
		_, err := source.LoadArtifacts(ctx, src, entry.PlainHTTP, cacheDir)
		if err != nil {
			log.Printf("  ERROR: %s: %v", src, err)
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", src, err))
			continue
		}
		log.Printf("  Pulled %s", src)
		result.Pulled++
	}

	return result, nil
}
