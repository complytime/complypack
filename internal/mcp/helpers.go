// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"fmt"

	"github.com/complytime/complypack/internal/requirement"
	"github.com/gemaraproj/go-gemara"
)

// findResolvedPolicy looks up a resolved policy by name. If no resolved
// policy is found, it falls back to wrapping a bare catalog via
// requirement.ResolveFromCatalog. Returns an error if neither a
// resolved policy nor a catalog is found.
func findResolvedPolicy(
	store *ResourceStore,
	name string,
) (*requirement.ResolvedPolicy, error) {
	if rp, found := store.resolved[name]; found {
		return rp, nil
	}

	art, ok := store.artifacts[name]
	if !ok {
		return nil, fmt.Errorf("policy or catalog %q not found", name)
	}
	cat, ok := art.(*gemara.ControlCatalog)
	if !ok {
		return nil, fmt.Errorf("policy or catalog %q not found", name)
	}

	rp, err := requirement.ResolveFromCatalog(name, cat)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve catalog %q: %w", name, err,
		)
	}
	return rp, nil
}
