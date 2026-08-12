// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"
)

// TestPolicyResult holds the aggregated results of a policy test run,
// including optional test-data validation.
type TestPolicyResult struct {
	TestDataValid  bool
	TestDataErrors []string
	TestsExecuted  bool
	Results        *TestResults
}

// TestPolicy runs a policy's test suite. It accepts optional
// pre-computed test-data validation errors; when present and
// non-empty, test execution is skipped.
func TestPolicy(
	ctx context.Context,
	eval Evaluator,
	policyContent string,
	testDataErrors []string,
) (*TestPolicyResult, error) {
	result := &TestPolicyResult{
		TestDataValid:  true,
		TestDataErrors: make([]string, 0),
		TestsExecuted:  false,
	}

	// If test-data validation failed, short-circuit
	if len(testDataErrors) > 0 {
		result.TestDataValid = false
		result.TestDataErrors = testDataErrors
		return result, nil
	}

	// Execute tests
	filename := "policy" + eval.FileExtension()
	files := map[string]string{
		filename: policyContent,
	}

	testResults, err := eval.Test(ctx, files)
	if err != nil {
		return nil, err
	}

	result.TestsExecuted = true
	result.Results = testResults
	if result.Results.Errors == nil {
		result.Results.Errors = make([]string, 0)
	}

	return result, nil
}
