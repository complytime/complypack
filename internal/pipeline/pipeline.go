// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/registry"
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

	// PolicySources maps each policy ID to the source string that
	// provided it. BuildProvenance uses this to distinguish OCI bundle
	// sources (whose authoritative provenance is the bundle reference)
	// from file sources (whose provenance is the MappingReference URLs).
	// When multiple sources provide the same policy ID, the last-declared
	// source wins. This is deterministic (iteration order of the sources
	// slice is stable) but means earlier sources' provenance is discarded.
	PolicySources map[string]string
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
	policySources := make(map[string]string)
	var loadErrs []error
	for _, entry := range sources {
		name := registry.RedactCredentials(entry.Source)
		src, err := source.LoadArtifacts(
			ctx, entry.Source, entry.PlainHTTP, cacheDir,
		)
		if err != nil {
			loadErrs = append(loadErrs, fmt.Errorf("source %s: %w", name, err))
			continue
		}
		for id := range src.Policies {
			policySources[id] = entry.Source
		}
		if err := loaded.Merge(src); err != nil {
			loadErrs = append(loadErrs, fmt.Errorf("source %s: %w", name, err))
			continue
		}
	}
	// Report all load/merge failures at once so every unreachable or
	// malformed source is named in a single joined error. A load failure
	// aborts the whole operation: no partial result is returned.
	if len(loadErrs) > 0 {
		return nil, errors.Join(loadErrs...)
	}

	resolved := make(map[string]*requirement.ResolvedPolicy)
	for id, policy := range loaded.Policies {
		rp, err := requirement.ResolvePolicy(*policy, loaded)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to resolve effective policy %s: %w",
				id, err,
			)
		}
		resolved[id] = rp
	}

	return &LoadResult{
		Artifacts:     loaded,
		Resolved:      resolved,
		PolicySources: policySources,
	}, nil
}
