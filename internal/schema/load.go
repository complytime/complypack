// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"context"
	"fmt"
	"log/slog"

	"cuelang.org/go/cue"
	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/schemas"
)

// LoadFromConfig loads platform schemas from configured schema
// references using the provided registry. If a schema ref has no
// explicit source, it checks the embedded schema index for a default.
func LoadFromConfig(
	ctx context.Context,
	schemaRefs []config.SchemaRef,
	reg *Registry,
) (map[string][]byte, map[string]cue.Value, error) {
	index, err := schemas.LoadIndex()
	if err != nil {
		return nil, nil, fmt.Errorf("loading schema index: %w", err)
	}

	schemaMap := make(map[string][]byte)
	cueSchemaMap := make(map[string]cue.Value)

	for _, ref := range schemaRefs {
		platform := ref.Platform
		source := schemas.ResolveSource(ref, index)

		s, err := reg.Load(ctx, source, platform)
		if err != nil {
			if source == "" {
				slog.Warn(
					"no schema available for platform, skipping",
					"platform", platform, "error", err,
				)
				continue
			}
			return nil, nil, fmt.Errorf(
				"failed to load schema for platform %s from %s: %w",
				platform, source, err,
			)
		}

		schemaMap[platform] = s.Bytes
		cueSchemaMap[platform] = s.CUE
		slog.Info("loaded schema", "platform", platform, "source", source)
	}

	return schemaMap, cueSchemaMap, nil
}
