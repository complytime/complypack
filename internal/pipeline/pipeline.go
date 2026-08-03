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
	var loadErrs []error
	for _, entry := range sources {
		name := SanitizeSourceID(entry.Source)
		src, err := source.LoadArtifacts(
			ctx, entry.Source, entry.PlainHTTP, cacheDir,
		)
		if err != nil {
			loadErrs = append(loadErrs, fmt.Errorf("source %s: %w", name, err))
			continue
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
		Artifacts: loaded,
		Resolved:  resolved,
	}, nil
}

// SanitizeSourceID strips any embedded credentials (userinfo) from a source
// identifier so that error messages naming a failed source never leak
// registry usernames or passwords declared in the source string
// (CWE-209: Information Exposure Through an Error Message).
//
// The path/host portion is preserved so the operator can still identify
// which source failed. Sources without recognizable userinfo (plain file
// paths, credential-free OCI references) are returned unchanged.
func SanitizeSourceID(source string) string {
	return registry.RedactCredentials(source)
}
