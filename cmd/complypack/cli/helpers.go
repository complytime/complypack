// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/complytime/complypack/internal/cache"
	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/pipeline"
	"github.com/complytime/complypack/internal/requirement"
)

// loadArtifacts loads and resolves artifacts from CLI flags
// or config file. This is the shared loading function used
// by all requirement analysis commands.
func loadArtifacts(
	ctx context.Context,
	sources []string,
	configPath string,
	rawCacheDir string,
) (*pipeline.LoadResult, error) {
	resolvedCacheDir, err := cache.ResolveDir(rawCacheDir)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve cache directory: %w", err,
		)
	}

	var entries []config.GemaraSourceEntry
	if len(sources) > 0 {
		entries, err = parseSourceFlags(sources)
		if err != nil {
			return nil, err
		}
	} else {
		cfgPath := configPath
		if cfgPath == "" {
			cfgPath = defaultConfigFilename
		}
		cfg, cfgErr := config.LoadConfig(
			cfgPath, false, os.Stderr,
		)
		if cfgErr != nil {
			return nil, fmt.Errorf(
				"failed to load config: %w", cfgErr,
			)
		}
		entries = cfg.Gemara.Sources
	}

	return pipeline.LoadAndResolve(
		ctx, entries, resolvedCacheDir,
	)
}

// findResolvedPolicyCLI performs the same two-step lookup as
// the MCP findResolvedPolicy helper: try resolved policies
// first, then fall back to bare catalog resolution.
func findResolvedPolicyCLI(
	lr *pipeline.LoadResult,
	name string,
) (*requirement.ResolvedPolicy, error) {
	if rp, found := lr.Resolved[name]; found {
		return rp, nil
	}

	cat, ok := lr.Artifacts.Catalogs[name]
	if !ok {
		return nil, fmt.Errorf(
			"catalog or policy %q not found in loaded artifacts",
			name,
		)
	}

	rp, err := requirement.ResolveFromCatalog(name, cat)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve catalog %q: %w", name, err,
		)
	}
	return rp, nil
}
