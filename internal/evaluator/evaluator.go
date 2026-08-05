// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"

	"cuelang.org/go/cue"

	"github.com/complytime/complypack/internal/testresult"
)

// Evaluator defines the interface for policy-language evaluators.
type Evaluator interface {
	// ID returns the unique identifier for this evaluator (e.g., "opa").
	ID() string

	// Validate checks policy syntax and compilation without executing.
	// Returns a list of validation errors (empty if valid).
	Validate(filename string, src string) []error

	// CheckContract validates that all input.* references in the policy
	// exist in the provided CUE schema.
	CheckContract(filename string, src string, schema cue.Value) ([]ContractViolation, error)

	// Test executes policy unit tests.
	// files is a map of filename -> source code.
	Test(ctx context.Context, files map[string]string) (*TestResults, error)

	// Lint performs static analysis and returns style/quality warnings.
	// Returns nil/nil if linting is not available or not applicable.
	Lint(filename string, src string) ([]LintWarning, error)

	// FileExtension returns the expected file extension for this evaluator's policies.
	FileExtension() string

	// RequiredFiles returns filenames that must exist in the content
	// directory alongside policy files for this evaluator to function
	// at scan time (e.g., "complytime-mapping.json" for OPA).
	// Returns nil if no additional files are required.
	RequiredFiles() []string
}

// ContractViolation represents a policy reference that doesn't exist in the schema.
type ContractViolation struct {
	Path     string // The input.* path that was referenced (e.g., "input.metadata.name")
	Location string // Location in the policy file (e.g., "policy.rego:12:5")
}

// Error implements the error interface.
func (v ContractViolation) Error() string {
	return v.Location + ": undefined reference: " + v.Path
}

// TestResults contains policy test execution results.
type TestResults struct {
	Total   int                 // Total number of tests
	Passed  int                 // Number of passing tests
	Failed  int                 // Number of failing tests
	Errors  []string            // Error messages from failing tests
	Details []testresult.Detail // Per-test detail for each non-skipped test
}

// LintWarning represents a non-fatal linting issue.
type LintWarning struct {
	Rule     string // Name of the lint rule that triggered
	Message  string // Human-readable warning message
	Location string // Location in the policy file (e.g., "policy.rego:12:5")
}
