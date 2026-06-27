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
		if !pathExistsInSchema(ref.path, schema) {
			violations = append(violations, ContractViolation{
				Path:     ref.path,
				Location: ref.location,
			})
		}
	}

	return violations, nil
}

// inputRef represents a reference to input.* in the policy.
type inputRef struct {
	path     string // The full path (e.g., "input.metadata.name")
	location string // Location in the policy file
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

		// Build the path string
		path := buildPath(ref)

		// Skip dynamic references like input[x] that can't be validated statically
		if strings.Contains(path, "[") {
			return false
		}

		refs = append(refs, inputRef{
			path:     path,
			location: ref[0].Location.String(),
		})

		return false
	})

	return refs
}

// buildPath constructs a dotted path from an AST reference.
func buildPath(ref ast.Ref) string {
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
			// Dynamic reference like input[x] - include the variable
			parts = append(parts, "["+string(v)+"]")
		default:
			// Other types (numbers, etc.) - convert to string
			parts = append(parts, fmt.Sprintf("[%v]", v))
		}
	}
	return strings.Join(parts, ".")
}

// pathExistsInSchema checks if a dotted path exists in the CUE schema.
// Uses a fallback chain: named/optional field -> pattern constraint -> CUE Allows.
func pathExistsInSchema(path string, schema cue.Value) bool {
	schemaPath := strings.TrimPrefix(path, "input.")
	if schemaPath == "input" {
		return true
	}

	parts := strings.Split(schemaPath, ".")
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
