// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/mod/modconfig"
	"github.com/complytime/complypack/schemas"
)

// loadCUESchemaForPlatform loads the CUE schema for a platform from embedded schemas.
func loadCUESchemaForPlatform(platform string) (cue.Value, error) {
	return loadEmbeddedCUESchema(platform)
}

// loadCUEFromSource loads a CUE schema from a parsed source.
func loadCUEFromSource(ctx context.Context, source SchemaSource, platform string) (cue.Value, error) {
	switch source.Type {
	case SourceTypeCUEModule:
		return loadFromCUERegistry(ctx, source.Path)

	case SourceTypeHTTPS, SourceTypeHTTP:
		data, format, err := fetchSchemaFromURL(ctx, source.Path)
		if err != nil {
			return cue.Value{}, err
		}
		if format != FormatCUE {
			return cue.Value{}, fmt.Errorf("expected CUE format, got %v", format)
		}
		return buildCUEFromBytes(data)

	case SourceTypeFile, SourceTypeLegacyPath:
		data, format, err := loadSchemaFromFile(source.Path)
		if err != nil {
			return cue.Value{}, err
		}
		if format != FormatCUE {
			return cue.Value{}, fmt.Errorf("expected CUE format, got %v", format)
		}
		return buildCUEFromBytes(data)

	case SourceTypeUnknown:
		// No source specified, use embedded
		return loadEmbeddedCUESchema(platform)

	default:
		return cue.Value{}, fmt.Errorf("unsupported source type: %v", source.Type)
	}
}

// loadFromCUERegistry loads a CUE module from the registry.
// The modulePath should be a CUE module path with optional version
// (e.g., "github.com/gemaraproj/gemara@latest" or "cue.dev/x/k8s.io/api/core/v1@latest").
// If no version is specified, "@latest" is appended.
func loadFromCUERegistry(_ context.Context, modulePath string) (cue.Value, error) {
	if !strings.Contains(modulePath, "@") {
		modulePath = modulePath + "@latest"
	}

	slog.Info("loading schema from CUE registry", "module", modulePath)

	reg, err := modconfig.NewRegistry(nil)
	if err != nil {
		return cue.Value{}, fmt.Errorf("creating CUE registry: %w", err)
	}

	instances := load.Instances([]string{modulePath}, &load.Config{
		Registry: reg,
	})
	if len(instances) == 0 {
		return cue.Value{}, fmt.Errorf("loading module %s: no instances returned", modulePath)
	}
	if err := instances[0].Err; err != nil {
		return cue.Value{}, fmt.Errorf("loading module %s: %w", modulePath, err)
	}

	cueCtx := cuecontext.New()
	val := cueCtx.BuildInstance(instances[0])
	if err := val.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("building schema: %w", err)
	}

	return val, nil
}

// loadEmbeddedCUESchema loads a CUE schema from embedded files.
func loadEmbeddedCUESchema(platform string) (cue.Value, error) {
	// Load CUE schema
	schemaBytes, err := schemas.GetBuiltInCUESchema(platform)
	if err != nil {
		return cue.Value{}, fmt.Errorf("failed to load CUE schema for %s: %w", platform, err)
	}

	// Parse CUE
	ctx := cuecontext.New()
	value := ctx.CompileBytes(schemaBytes)
	if value.Err() != nil {
		return cue.Value{}, fmt.Errorf("failed to compile CUE schema for %s: %w", platform, value.Err())
	}

	return value, nil
}
