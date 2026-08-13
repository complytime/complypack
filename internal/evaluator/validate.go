// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"cuelang.org/go/cue"
)

// ValidatePolicyResult holds the aggregated results of a full policy
// validation pass: syntax, contract, and lint.
type ValidatePolicyResult struct {
	Valid              bool
	SyntaxErrors       []string
	ContractViolations []ContractViolation
	LintWarnings       []LintWarning
	LintErr            error
}

// ValidatePolicy runs the full validation pipeline against a policy
// source string: syntax validation, contract checking against the
// provided CUE schema, and linting.
//
// Contract and lint checks are skipped when syntax errors exist.
// Lint errors are captured but do not affect the Valid verdict.
func ValidatePolicy(
	eval Evaluator,
	policyContent string,
	cueSchema cue.Value,
) *ValidatePolicyResult {
	filename := "policy" + eval.FileExtension()

	// Syntax validation
	syntaxErrs := eval.Validate(filename, policyContent)

	result := &ValidatePolicyResult{
		SyntaxErrors:       make([]string, 0, len(syntaxErrs)),
		ContractViolations: make([]ContractViolation, 0),
		LintWarnings:       make([]LintWarning, 0),
	}

	for _, e := range syntaxErrs {
		result.SyntaxErrors = append(
			result.SyntaxErrors, e.Error(),
		)
	}

	// Contract and lint checks (only when syntax is valid)
	if len(syntaxErrs) == 0 {
		violations, err := eval.CheckContract(
			filename, policyContent, cueSchema,
		)
		if err != nil {
			// Contract check infrastructure failure is
			// treated as a syntax error for reporting.
			result.SyntaxErrors = append(
				result.SyntaxErrors,
				"contract check failed: "+err.Error(),
			)
		} else {
			result.ContractViolations = violations
		}

		// Lint (graceful degradation)
		warnings, lintErr := eval.Lint(
			filename, policyContent,
		)
		result.LintWarnings = warnings
		result.LintErr = lintErr
	}

	result.Valid = len(result.SyntaxErrors) == 0 &&
		len(result.ContractViolations) == 0

	return result
}
