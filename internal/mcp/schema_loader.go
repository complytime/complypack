// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/mod/modconfig"
	"cuelang.org/go/mod/modregistry"
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
		return loadEmbeddedCUESchema(platform)

	default:
		return cue.Value{}, fmt.Errorf("unsupported source type: %v", source.Type)
	}
}

// loadFromCUERegistry loads a CUE module from the Central Registry by creating
// a temporary CUE module workspace that declares the target as a dependency.
// The CUE SDK's load.Instances requires a local module context; it does not
// support passing remote module paths directly.
func loadFromCUERegistry(ctx context.Context, modulePath string) (cue.Value, error) {
	modPath, version := splitModuleVersion(modulePath)

	slog.Info("loading schema from CUE registry", "module", modPath, "requestedVersion", version)

	resolver, err := modconfig.NewResolver(nil)
	if err != nil {
		return cue.Value{}, fmt.Errorf("creating CUE resolver: %w", err)
	}

	if version == "" || version == "latest" {
		resolved, resolveErr := resolveLatestVersion(ctx, modPath, resolver)
		if resolveErr != nil {
			return cue.Value{}, fmt.Errorf("resolving latest version for %s: %w", modPath, resolveErr)
		}
		version = resolved
		slog.Info("resolved latest version", "module", modPath, "version", version)
	}

	tmpDir, err := os.MkdirTemp("", "complypack-cue-*")
	if err != nil {
		return cue.Value{}, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := writeCUEWorkspace(tmpDir, modPath, version); err != nil {
		return cue.Value{}, fmt.Errorf("writing temp CUE workspace: %w", err)
	}

	reg, err := modconfig.NewRegistry(nil)
	if err != nil {
		return cue.Value{}, fmt.Errorf("creating CUE registry: %w", err)
	}

	importPath := importPathForModule(modPath)

	instances := load.Instances([]string{importPath}, &load.Config{
		Dir:      tmpDir,
		Registry: reg,
	})
	if len(instances) == 0 {
		return cue.Value{}, fmt.Errorf("loading module %s: no instances returned", modPath)
	}
	if err := instances[0].Err; err != nil {
		return cue.Value{}, fmt.Errorf("loading module %s@%s: %w", modPath, version, err)
	}

	cueCtx := cuecontext.New()
	val := cueCtx.BuildInstance(instances[0])
	if err := val.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("building schema from %s@%s: %w", modPath, version, err)
	}

	return val, nil
}

// splitModuleVersion separates a module path from its version.
// Input formats:
//   - "cue.dev/x/githubactions" → ("cue.dev/x/githubactions", "")
//   - "cue.dev/x/githubactions@v0.2.0" → ("cue.dev/x/githubactions", "v0.2.0")
//   - "cue.dev/x/githubactions@latest" → ("cue.dev/x/githubactions", "latest")
//
// Note: CUE module paths may contain a major version suffix (e.g., "@v0")
// as part of the path identity. This function splits on the LAST "@" to
// handle both "mod@v0" (path only) and "mod@v0.2.0" (path + version).
// A bare major like "v0" without minor/patch is treated as part of the path.
func splitModuleVersion(input string) (string, string) {
	idx := strings.LastIndex(input, "@")
	if idx < 0 {
		return input, ""
	}

	path := input[:idx]
	version := input[idx+1:]

	// If the version is just a major (e.g., "v0") it's part of the module
	// identity, not a resolvable version. Treat as path-only.
	if isMajorOnly(version) {
		return input, ""
	}

	return path, version
}

// isMajorOnly returns true if v matches "v0", "v1", "v2", etc. without
// minor or patch components.
func isMajorOnly(v string) bool {
	if !strings.HasPrefix(v, "v") {
		return false
	}
	rest := v[1:]
	if rest == "" {
		return false
	}
	for _, c := range rest {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// resolveLatestVersion queries the CUE registry for available versions of a
// module and returns the latest one. The module path should include the major
// version suffix (e.g., "cue.dev/x/githubactions@v0").
func resolveLatestVersion(ctx context.Context, modPath string, resolver *modconfig.Resolver) (string, error) {
	client := modregistry.NewClientWithResolver(resolver)

	versions, err := client.ModuleVersions(ctx, modPath)
	if err != nil {
		return "", fmt.Errorf("listing versions: %w", err)
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("no versions found for %s", modPath)
	}

	// ModuleVersions returns versions sorted in semver order; last is latest.
	return versions[len(versions)-1], nil
}

// writeCUEWorkspace creates a minimal CUE module workspace in dir that
// declares the target module as a dependency. load.Instances is then called
// with the target's import path (not ".") to load the remote package directly.
func writeCUEWorkspace(dir, modPath, version string) error {
	modDir := filepath.Join(dir, "cue.mod")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		return err
	}

	depKey := modPath
	if !strings.Contains(modPath, "@") {
		depKey = modPath + "@v0"
	}

	moduleCUE := fmt.Sprintf(`module: "complypack.local/schema@v0"
language: version: "v0.16.1"
deps: "%s": v: "%s"
`, depKey, version)

	return os.WriteFile(filepath.Join(modDir, "module.cue"), []byte(moduleCUE), 0600)
}

// importPathForModule returns the CUE import path for a module path.
// If the path has a major version suffix (@v0, @v2), it's stripped for the
// import because CUE treats the path without the suffix as the import path
// for the default major version.
func importPathForModule(modPath string) string {
	if idx := strings.LastIndex(modPath, "@"); idx > 0 {
		suffix := modPath[idx+1:]
		if isMajorOnly(suffix) {
			return modPath[:idx]
		}
	}
	return modPath
}

// loadEmbeddedCUESchema loads a CUE schema from embedded files.
func loadEmbeddedCUESchema(platform string) (cue.Value, error) {
	schemaBytes, err := schemas.GetBuiltInCUESchema(platform)
	if err != nil {
		return cue.Value{}, fmt.Errorf("failed to load CUE schema for %s: %w", platform, err)
	}

	ctx := cuecontext.New()
	value := ctx.CompileBytes(schemaBytes)
	if value.Err() != nil {
		return cue.Value{}, fmt.Errorf("failed to compile CUE schema for %s: %w", platform, value.Err())
	}

	return value, nil
}
