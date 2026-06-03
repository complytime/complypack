// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"log/slog"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/mod/modconfig"
	"github.com/complytime/complypack/schemas"
)

// platformModulePaths maps our platform names to CUE registry module paths.
var platformModulePaths = map[string]string{
	"kubernetes": "cue.dev/x/k8s.io/api/core/v1",
	// TODO: Add other platforms when available in CUE registry
	// "terraform":  "...",
	// "docker":     "...",
	// "ansible":    "...",
	// "ci":         "...",
}

// loadCUESchemaForPlatform loads the CUE schema for a platform.
// Tries configured source first, then hardcoded registry paths, then embedded.
func loadCUESchemaForPlatform(platform string) (cue.Value, error) {
	return loadCUESchemaForPlatformWithSource(platform, "")
}

// loadCUESchemaForPlatformWithSource loads a CUE schema from the specified source.
// If source is empty, uses hardcoded registry paths, then falls back to embedded.
func loadCUESchemaForPlatformWithSource(platform, source string) (cue.Value, error) {
	// Validate platform is in our known list (for fallback purposes)
	validPlatform := false
	for _, p := range schemas.BuiltInPlatforms {
		if p == platform {
			validPlatform = true
			break
		}
	}
	if !validPlatform {
		return cue.Value{}, fmt.Errorf("unsupported platform %q (available: %v)", platform, schemas.BuiltInPlatforms)
	}

	// Try configured source first
	if source != "" {
		parsed, err := ParseSchemaSource(source)
		if err != nil {
			slog.Warn("failed to parse schema source, falling back", "platform", platform, "source", source, "error", err)
		} else {
			val, err := loadCUEFromSource(context.Background(), parsed, platform)
			if err == nil {
				slog.Info("successfully loaded schema from configured source", "platform", platform, "source", source)
				return val, nil
			}
			slog.Warn("failed to load from configured source, falling back", "platform", platform, "source", source, "error", err)
		}
	}

	// Try hardcoded CUE registry path if known
	if modulePath, ok := platformModulePaths[platform]; ok {
		slog.Info("attempting to load schema from CUE registry", "platform", platform, "module", modulePath)
		val, err := loadFromCUERegistry(context.Background(), modulePath)
		if err == nil {
			slog.Info("successfully loaded schema from CUE registry", "platform", platform)
			return val, nil
		}
		slog.Warn("failed to load from CUE registry, falling back to embedded", "platform", platform, "error", err)
	}

	// Fallback to embedded schema
	slog.Info("loading embedded schema", "platform", platform)
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
func loadFromCUERegistry(ctx context.Context, modulePath string) (cue.Value, error) {
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
