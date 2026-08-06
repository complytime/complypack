// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"strings"

	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/schemas/jsonschema"
	"gopkg.in/yaml.v3"
)

// buildConfigFromFlags creates a ComplyPackConfig from --source and --schema flag values.
func buildConfigFromFlags(sources, schemas []string) (*config.ComplyPackConfig, error) {
	entries, err := parseSourceFlags(sources)
	if err != nil {
		return nil, err
	}

	schemaRefs, err := parseSchemaFlags(schemas)
	if err != nil {
		return nil, err
	}

	cfg := config.BuildConfig("", "", "", entries, schemaRefs)

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshaling config: %w", err)
	}

	if _, err := jsonschema.ValidateConfig(data, false); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

// parseSourceFlags converts --source flag values into GemaraSourceEntry values.
//
//   - oci://...        -> GemaraSourceEntry{Source: "oci://...", PlainHTTP: false}
//   - oci+http://...   -> GemaraSourceEntry{Source: "oci://...", PlainHTTP: true}
func parseSourceFlags(sources []string) ([]config.GemaraSourceEntry, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	entries := make([]config.GemaraSourceEntry, 0, len(sources))
	for _, s := range sources {
		if s == "" {
			return nil, fmt.Errorf("empty source flag value")
		}

		entry := config.GemaraSourceEntry{}
		if strings.HasPrefix(s, "oci+http://") {
			entry.Source = "oci://" + strings.TrimPrefix(s, "oci+http://")
			entry.PlainHTTP = true
		} else {
			entry.Source = s
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// buildSchemaRefs constructs schema references for schema loading.
// If --schema flags are provided, they are parsed. Otherwise, a
// default ref for the given platform is created (relying on the
// embedded schema index for resolution).
func buildSchemaRefs(
	platform string, schemaFlags []string,
) ([]config.SchemaRef, error) {
	if len(schemaFlags) > 0 {
		return parseSchemaFlags(schemaFlags)
	}
	return []config.SchemaRef{{Platform: platform}}, nil
}

// parseSchemaFlags converts --schema flag values into SchemaRef values.
//
//   - "kubernetes"                        -> SchemaRef{Platform: "kubernetes"} (embedded)
//   - "ci=cue://cue.dev/x/actions@v0"    -> SchemaRef{Platform: "ci", Source: "cue://..."}
func parseSchemaFlags(schemas []string) ([]config.SchemaRef, error) {
	if len(schemas) == 0 {
		return nil, nil
	}

	refs := make([]config.SchemaRef, 0, len(schemas))
	for _, s := range schemas {
		if s == "" {
			return nil, fmt.Errorf("empty schema flag value")
		}

		ref := config.SchemaRef{}
		if idx := strings.IndexByte(s, '='); idx >= 0 {
			ref.Platform = s[:idx]
			ref.Source = s[idx+1:]
			if ref.Platform == "" {
				return nil, fmt.Errorf("empty platform name in schema flag %q", s)
			}
			if ref.Source == "" {
				return nil, fmt.Errorf(
					"empty source for platform %q in schema flag %q",
					ref.Platform, s,
				)
			}
		} else {
			ref.Platform = s
		}
		refs = append(refs, ref)
	}
	return refs, nil
}
