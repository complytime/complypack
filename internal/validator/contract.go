// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"github.com/open-policy-agent/opa/v1/ast"
)

// ContractViolation represents a policy reference that doesn't exist in the schema.
type ContractViolation struct {
	Path     string // The input.* path that was referenced (e.g., "input.metadata.name")
	Location string // Location in the policy file (e.g., "policy.rego:12:5")
}

// Error implements the error interface.
func (v ContractViolation) Error() string {
	return v.Location + ": undefined reference: " + v.Path
}

// CheckContract validates that all input.* references in a Rego policy
// exist in the provided CUE schema.
// Returns a list of contract violations (empty if all references are valid).
func CheckContract(filename string, src string, schema cue.Value) ([]ContractViolation, error) {
	// Parse the Rego policy
	mod, err := ast.ParseModuleWithOpts(filename, src, ast.ParserOptions{RegoVersion: ast.RegoV1})
	if err != nil {
		return nil, fmt.Errorf("failed to parse Rego: %w", err)
	}

	// Extract all input.* references
	inputRefs := extractInputRefs(mod)

	// Check each reference against the schema
	var violations []ContractViolation
	for _, ref := range inputRefs {
		if !pathExistsInSchema(ref.segments, schema) {
			violations = append(violations, ContractViolation{
				Path:     ref.rawPath,
				Location: ref.location,
			})
		}
	}

	return violations, nil
}

// inputRef represents a reference to input.* in the policy.
type inputRef struct {
	segments []string // Structured path segments (e.g., ["input", "metadata", "name"])
	rawPath  string   // Display path (e.g., "input.metadata.name" or `input.metadata.annotations["cert-manager.io/duration"]`)
	location string   // Location in the policy file
}

// extractInputRefs walks the AST and extracts all input.* references.
func extractInputRefs(mod *ast.Module) []inputRef {
	var refs []inputRef

	ast.WalkRefs(mod, func(ref ast.Ref) bool {
		// Only process refs that start with "input"
		if len(ref) == 0 {
			return false
		}

		first, ok := ref[0].Value.(ast.Var)
		if !ok || string(first) != "input" {
			return false
		}

		// Build the structured path; nil means dynamic reference (has ast.Var terms).
		segments := buildPath(ref)
		if segments == nil {
			return false
		}

		refs = append(refs, inputRef{
			segments: segments,
			rawPath:  formatPath(segments),
			location: ref[0].Location.String(),
		})

		return false
	})

	return refs
}

// buildPath constructs a structured path from an AST reference.
// Each AST term becomes exactly one slice element, preserving
// dotted string keys (e.g., "cert-manager.io/duration") as atomic segments.
// Returns nil if any term (after the first) is a variable (dynamic reference).
func buildPath(ref ast.Ref) []string {
	var parts []string
	for i, term := range ref {
		if i == 0 {
			// First term is always "input"
			parts = append(parts, "input")
			continue
		}

		switch v := term.Value.(type) {
		case ast.String:
			parts = append(parts, string(v))
		case ast.Var:
			// Dynamic reference like input[x] — cannot be validated statically.
			return nil
		default:
			// Other types (numbers, etc.) — cannot be validated statically.
			return nil
		}
	}
	return parts
}

// formatPath reconstructs a display string from structured path segments.
// Uses dot notation for simple keys and bracket notation for keys containing dots.
func formatPath(segments []string) string {
	var b strings.Builder
	for i, seg := range segments {
		if i == 0 {
			b.WriteString(seg)
			continue
		}
		if strings.Contains(seg, ".") {
			b.WriteString(`["`)
			b.WriteString(seg)
			b.WriteString(`"]`)
		} else {
			b.WriteByte('.')
			b.WriteString(seg)
		}
	}
	return b.String()
}

// pathExistsInSchema checks if a structured path exists in the CUE schema.
// segments should include "input" as the first element.
// Uses a fallback chain: named/optional field -> pattern constraint -> CUE Allows.
func pathExistsInSchema(segments []string, schema cue.Value) bool {
	if len(segments) <= 1 {
		// Just "input" or empty — always valid.
		return true
	}

	// Skip the leading "input" segment.
	parts := segments[1:]
	current := schema

	for _, part := range parts {
		next := current.LookupPath(cue.MakePath(cue.Str(part).Optional()))
		if next.Exists() {
			current = next
			continue
		}

		next = current.LookupPath(cue.MakePath(cue.AnyString))
		if next.Exists() {
			current = next
			continue
		}

		// Delegate to CUE: handles top type (_), disjunctions, and
		// other structural allowances. Since we have no value type to
		// continue walking, accept the entire remaining path.
		if current.Allows(cue.Str(part)) {
			return true
		}

		return false
	}

	return true
}
