// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"
	"fmt"

	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/requirement"
	"github.com/complytime/complypack/internal/source"
)

// LoadResult holds the artifacts and resolved policies produced by
// LoadAndResolve.
type LoadResult struct {
	// Artifacts is the merged set of all Gemara artifacts loaded
	// from configured sources.
	Artifacts *requirement.ArtifactSet

	// Resolved maps policy IDs to their fully resolved policies.
	Resolved map[string]*requirement.ResolvedPolicy
}

// LoadAndResolve loads Gemara artifacts from all configured sources,
// merges them into a single ArtifactSet, and resolves all policies
// against the merged set.
func LoadAndResolve(
	ctx context.Context,
	sources []config.GemaraSourceEntry,
	cacheDir string,
) (*LoadResult, error) {
	loaded := requirement.NewArtifactSet()
	for _, entry := range sources {
		src, err := source.LoadArtifacts(
			ctx, entry.Source, entry.PlainHTTP, cacheDir,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to load artifacts from %s: %w",
				entry.Source, err,
			)
		}
		if err := loaded.Merge(src); err != nil {
			return nil, fmt.Errorf(
				"failed to merge artifacts from %s: %w",
				entry.Source, err,
			)
		}
	}

	resolved := make(map[string]*requirement.ResolvedPolicy)
	for id, policy := range loaded.Policies {
		if len(loaded.Catalogs) > 0 || len(loaded.Guidance) > 0 {
			rp, err := requirement.ResolvePolicy(*policy, loaded)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to resolve effective policy %s: %w",
					id, err,
				)
			}
			resolved[id] = rp
		}
	}

	return &LoadResult{
		Artifacts: loaded,
		Resolved:  resolved,
	}, nil
}
