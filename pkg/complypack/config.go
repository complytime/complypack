// SPDX-License-Identifier: Apache-2.0

package complypack

import (
	"fmt"

	"github.com/complytime/complypack/schemas/jsonschema"
)

// Config is the ComplyPack OCI artifact configuration.
// Embedded in the OCI config layer so consumers can identify the pack
// and route it to the correct provider without inspecting the content.
type Config struct {
	// ID is the globally unique pack identifier using reverse-domain convention
	// (e.g., "io.complytime.my-controls", "com.acme.appsec").
	// Required. Survives registry moves and distinguishes packs from different
	// authors that target the same evaluator.
	ID string `json:"id"`

	// EvaluatorID identifies the provider plugin that evaluates this pack's
	// content (e.g., "opa"). Must match the provider's registered ID.
	// Required. Used by complyctl to dispatch content to the correct provider.
	EvaluatorID string `json:"evaluator-id"`

	// Version is the ComplyPack artifact version.
	// Required. Semantic versioning recommended.
	Version string `json:"version"`

	// Source links this ComplyPack to the Gemara content it implements.
	// Optional. Empty for standalone policies. One entry per resolved policy.
	Source []Provenance `json:"source,omitempty"`
}

// Provenance links a ComplyPack to a single Gemara policy and the Gemara
// content that policy imports.
type Provenance struct {
	// PolicyID identifies the resolved Gemara policy this pack implements.
	PolicyID string `json:"policy-id"`

	// GemaraContent is the set of Gemara references (catalogs and guidance)
	// the policy imports. Non-empty.
	GemaraContent []GemaraRef `json:"gemara-content"`
}

// GemaraRef records a single Gemara reference (catalog or guidance) imported
// by a policy. URI values are recorded into the published OCI config blob, so
// they are sanitized upstream (userinfo/query/fragment stripped, local
// filesystem paths omitted) before reaching this struct.
type GemaraRef struct {
	// URI locates the Gemara content (e.g. an OCI reference). May be empty for
	// local or url-less references. Sanitized before recording.
	URI string `json:"uri,omitempty"`

	// Version is the referenced content version. Optional.
	Version string `json:"version,omitempty"`

	// ReferenceID is the import identifier within the policy. Required.
	ReferenceID string `json:"reference-id"`
}

// Validate checks that required Config fields are present and well-formed.
// Format patterns are derived from the embedded JSON Schema — same source
// of truth used by the YAML config layer.
// Returns ErrInvalidConfig if validation fails.
func (c Config) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidConfig)
	}
	if !jsonschema.IDPattern().MatchString(c.ID) {
		return fmt.Errorf("%w: id %q must use reverse-domain notation (e.g. io.complytime.my-pack)", ErrInvalidConfig, c.ID)
	}
	if c.EvaluatorID == "" {
		return fmt.Errorf("%w: evaluator-id is required", ErrInvalidConfig)
	}
	if !jsonschema.EvaluatorIDPattern().MatchString(c.EvaluatorID) {
		return fmt.Errorf("%w: evaluator-id %q must be a lowercase identifier (e.g. opa)", ErrInvalidConfig, c.EvaluatorID)
	}
	if c.Version == "" {
		return fmt.Errorf("%w: version is required", ErrInvalidConfig)
	}
	if !jsonschema.VersionPattern().MatchString(c.Version) {
		return fmt.Errorf("%w: version %q must be semver (e.g. 1.0.0)", ErrInvalidConfig, c.Version)
	}
	for i, prov := range c.Source {
		if prov.PolicyID == "" {
			return fmt.Errorf("%w: source[%d].policy-id is required", ErrInvalidConfig, i)
		}
		if len(prov.GemaraContent) == 0 {
			return fmt.Errorf("%w: source[%d].gemara-content is required when source is set", ErrInvalidConfig, i)
		}
		for j, ref := range prov.GemaraContent {
			if ref.ReferenceID == "" {
				return fmt.Errorf("%w: source[%d].gemara-content[%d].reference-id is required", ErrInvalidConfig, i, j)
			}
		}
	}
	return nil
}
