// SPDX-License-Identifier: Apache-2.0

package complypack

import "fmt"

// Config is the ComplyPack OCI artifact configuration.
// It identifies which evaluator consumes this pack and tracks optional provenance.
type Config struct {
	// EvaluatorID identifies the policy evaluator (e.g., "io.complytime.opa").
	// Required. Used by consumers to dispatch to the correct evaluator plugin.
	EvaluatorID string `json:"evaluator-id"`

	// Version is the ComplyPack artifact version.
	// Required. Semantic versioning recommended.
	Version string `json:"version"`

	// Source links this ComplyPack to the Gemara content it implements.
	// Optional. Nil for standalone policies.
	Source *Provenance `json:"source,omitempty"`
}

// Provenance links a ComplyPack to the Gemara content and policy it implements.
type Provenance struct {
	// GemaraContent is the URI or hash of the Gemara catalog.
	// Examples: "oci://registry/gemara/controls:latest", "sha256:abc123..."
	GemaraContent string `json:"gemara-content"`

	// PolicyID identifies the policy within the Gemara catalog.
	PolicyID string `json:"policy-id"`
}

// Validate checks that required Config fields are present.
// Returns ErrInvalidConfig if validation fails.
func (c Config) Validate() error {
	if c.EvaluatorID == "" {
		return fmt.Errorf("%w: evaluator-id is required", ErrInvalidConfig)
	}
	if c.Version == "" {
		return fmt.Errorf("%w: version is required", ErrInvalidConfig)
	}
	if c.Source != nil {
		if c.Source.GemaraContent == "" {
			return fmt.Errorf("%w: source.gemara-content is required when source is set", ErrInvalidConfig)
		}
		if c.Source.PolicyID == "" {
			return fmt.Errorf("%w: source.policy-id is required when source is set", ErrInvalidConfig)
		}
	}
	return nil
}
