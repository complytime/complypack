// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"context"
	"fmt"
)

// Registry holds an ordered list of loaders and dispatches to the first match.
type Registry struct {
	loaders []Loader
}

// NewRegistry creates a Registry with the given loaders.
func NewRegistry(loaders ...Loader) *Registry {
	return &Registry{loaders: loaders}
}

// DefaultRegistry returns a registry with all built-in loaders in priority order.
func DefaultRegistry() *Registry {
	return NewRegistry(
		&CUELoader{},
		&URLLoader{},
		&FileLoader{},
		&LegacyLoader{},
	)
}

// Load finds the first matching loader and delegates to it.
func (r *Registry) Load(ctx context.Context, source string, platform string) (*Schema, error) {
	for _, l := range r.loaders {
		if l.Match(source) {
			return l.Load(ctx, source, platform)
		}
	}
	return nil, fmt.Errorf("no loader matched source %q", source)
}
