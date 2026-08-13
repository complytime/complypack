// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/internal/schema"
	"github.com/spf13/cobra"
)

// errTestsFailed is returned when test execution
// completes but tests failed or test data was invalid.
// It triggers a non-zero exit code.
var errTestsFailed = errors.New("policy tests failed")

// testPolicyRunParams holds parsed CLI parameters
// for the test-policy command.
type testPolicyRunParams struct {
	policyFile   string
	platform     string
	evalID       string
	schemas      []string
	testDataFile string
	format       string
	stdout       io.Writer
}

func testPolicyCmd() *cobra.Command {
	var (
		platform     string
		evalID       string
		schemas      []string
		testDataFile string
		format       string
	)

	cmd := &cobra.Command{
		Use:   "test-policy <file>",
		Short: "Execute policy tests with optional test-data schema validation",
		Long: `Execute a policy file's test suite.

When --test-data is provided, the test data JSON file is first validated against
the platform's CUE schema. If validation fails, tests are not executed.

When --test-data is omitted, test-data validation is skipped and tests run
directly against the policy file.

Output formats:
  human  Styled output with Unicode symbols and color (default for terminals)
  text   Plain bracketed labels for CI/grep
  json   Structured JSON matching the MCP test_policy tool response

Exit codes:
  0  All tests pass (or no tests executed)
  1  Tests failed or test data validation failed

Examples:
  complypack test-policy policy.rego --platform kubernetes-deployment
  complypack test-policy policy.rego --platform kubernetes-deployment --test-data fixtures.json
  complypack test-policy policy.rego --platform kubernetes-deployment --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedFormat, err := resolveFormat(format)
			if err != nil {
				return err
			}
			return runTestPolicy(cmd.Context(), testPolicyRunParams{
				policyFile:   args[0],
				platform:     platform,
				evalID:       evalID,
				schemas:      schemas,
				testDataFile: testDataFile,
				format:       resolvedFormat,
				stdout:       cmd.OutOrStdout(),
			})
		},
	}

	cmd.Flags().StringVar(&platform, "platform", "",
		"Target platform for test-data schema validation (required)")
	cmd.Flags().StringVar(&evalID, "evaluator", "",
		"Evaluator ID (auto-detected if omitted)")
	cmd.Flags().StringArrayVar(&schemas, "schema", nil,
		"Platform schema override (repeatable, e.g., platform=source)")
	cmd.Flags().StringVar(&testDataFile, "test-data", "",
		"Path to JSON test-data file for CUE schema pre-validation (omit to skip)")
	cmd.Flags().StringVarP(&format, "format", "f", "",
		"Output format: human, text, or json (default: auto-detected)")

	_ = cmd.MarkFlagRequired("platform")

	_ = cmd.RegisterFlagCompletionFunc("format",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return []string{formatHuman, formatText, formatJSON},
				cobra.ShellCompDirectiveNoFileComp
		})

	return cmd
}

func runTestPolicy(
	ctx context.Context,
	params testPolicyRunParams,
) error {
	// Read policy file
	content, err := os.ReadFile(params.policyFile)
	if err != nil {
		return fmt.Errorf("reading policy file: %w", err)
	}

	// Validate test data against CUE schema when provided
	var testDataErrors []string
	if params.testDataFile != "" {
		tdErrs, err := validateTestDataFromFile(
			ctx, params,
		)
		if err != nil {
			return err
		}
		testDataErrors = tdErrs
	}

	// Resolve evaluator
	eval, err := evaluator.DefaultRegistry().Resolve(
		params.evalID,
	)
	if err != nil {
		return fmt.Errorf("resolving evaluator: %w", err)
	}

	// Delegate to domain function
	domainResult, err := evaluator.TestPolicy(
		ctx, eval, string(content), testDataErrors,
	)
	if err != nil {
		return fmt.Errorf("test execution failed: %w", err)
	}

	if err := writeTestPolicyOutput(
		params.format, params.stdout, domainResult,
	); err != nil {
		return err
	}

	if !domainResult.TestDataValid || hasFailed(domainResult) {
		return errTestsFailed
	}
	return nil
}

// hasFailed returns true when test results contain
// failures or errors.
func hasFailed(r *evaluator.TestPolicyResult) bool {
	if !r.TestsExecuted || r.Results == nil {
		return false
	}
	return r.Results.Failed > 0 ||
		len(r.Results.Errors) > 0
}

// validateTestDataFromFile reads a JSON test-data file,
// loads the platform CUE schema, and validates the data
// using schema.ValidateData. Returns validation errors
// (if any) or an error for infrastructure failures.
func validateTestDataFromFile(
	ctx context.Context,
	params testPolicyRunParams,
) ([]string, error) {
	testDataBytes, err := os.ReadFile(params.testDataFile)
	if err != nil {
		return nil, fmt.Errorf(
			"reading test-data file: %w", err,
		)
	}

	var testData map[string]interface{}
	if err := json.Unmarshal(
		testDataBytes, &testData,
	); err != nil {
		return nil, fmt.Errorf(
			"parsing test-data JSON: %w", err,
		)
	}

	schemaRefs, err := buildSchemaRefs(
		params.platform, params.schemas,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"building schema refs: %w", err,
		)
	}

	_, cueSchemas, err := schema.LoadFromConfig(
		ctx, schemaRefs, schema.DefaultRegistry(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"loading schemas: %w", err,
		)
	}

	cueSchema, ok := cueSchemas[params.platform]
	if !ok {
		return nil, fmt.Errorf(
			"no schema found for platform %q;"+
				" provide --schema %s=<source>",
			params.platform, params.platform,
		)
	}

	return schema.ValidateData(testData, cueSchema), nil
}

func writeTestPolicyOutput(
	format string,
	w io.Writer,
	result *evaluator.TestPolicyResult,
) error {
	switch format {
	case formatJSON:
		return writeTestPolicyJSON(w, result)
	case formatText:
		return writeTestPolicyText(w, result)
	case formatHuman:
		return writeTestPolicyHuman(w, result)
	default:
		return fmt.Errorf(
			"unknown format %q;"+
				" valid formats: human, text, json",
			format,
		)
	}
}

func writeTestPolicyJSON(
	w io.Writer,
	result *evaluator.TestPolicyResult,
) error {
	response := map[string]interface{}{
		"testDataValid":  result.TestDataValid,
		"testDataErrors": result.TestDataErrors,
		"testsExecuted":  result.TestsExecuted,
	}
	if result.Results != nil {
		response["results"] = map[string]interface{}{
			"total":  result.Results.Total,
			"passed": result.Results.Passed,
			"failed": result.Results.Failed,
			"errors": result.Results.Errors,
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(response)
}

func writeTestPolicyText(
	w io.Writer,
	result *evaluator.TestPolicyResult,
) error {
	if !result.TestDataValid {
		fmt.Fprintln(w,
			"[FAIL] Test data validation failed")
		for _, e := range result.TestDataErrors {
			fmt.Fprintf(w,
				"  [TEST DATA ERROR] %s\n", e)
		}
		return nil
	}

	if !result.TestsExecuted {
		fmt.Fprintln(w,
			"[SKIP] Tests were not executed")
		return nil
	}

	r := result.Results
	if r.Failed == 0 && len(r.Errors) == 0 {
		fmt.Fprintf(w, "[PASS] %d/%d tests passed\n",
			r.Passed, r.Total)
	} else {
		fmt.Fprintf(w,
			"[FAIL] %d/%d tests passed, %d failed\n",
			r.Passed, r.Total, r.Failed)
	}

	for _, e := range r.Errors {
		fmt.Fprintf(w, "  [ERROR] %s\n", e)
	}

	return nil
}

func writeTestPolicyHuman(
	w io.Writer,
	result *evaluator.TestPolicyResult,
) error {
	if !result.TestDataValid {
		fmt.Fprintln(w,
			styleFail.Render(
				"✗ Test data validation failed"))
		fmt.Fprintln(w)
		fmt.Fprintln(w,
			styleTitle.Render("Test Data Errors"))
		for _, e := range result.TestDataErrors {
			fmt.Fprintf(w, "  %s %s\n",
				styleFail.Render("✗"), e)
		}
		return nil
	}

	if !result.TestsExecuted {
		fmt.Fprintln(w,
			styleDim.Render(
				"Tests were not executed"))
		return nil
	}

	r := result.Results
	if r.Failed == 0 && len(r.Errors) == 0 {
		fmt.Fprintln(w, stylePass.Render(
			fmt.Sprintf("✓ %d/%d tests passed",
				r.Passed, r.Total)))
	} else {
		fmt.Fprintln(w, styleFail.Render(
			fmt.Sprintf(
				"✗ %d/%d tests passed, %d failed",
				r.Passed, r.Total, r.Failed)))
	}

	if len(r.Errors) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w,
			styleTitle.Render("Test Errors"))
		for _, e := range r.Errors {
			fmt.Fprintf(w, "  %s %s\n",
				styleFail.Render("✗"), e)
		}
	}

	return nil
}
