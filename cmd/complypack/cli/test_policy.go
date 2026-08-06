// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/internal/schema"
	"github.com/spf13/cobra"
)

// testPolicyRunParams holds parsed CLI parameters for the test-policy command.
type testPolicyRunParams struct {
	policyFile   string
	platform     string
	evalID       string
	schemas      []string
	testDataFile string
	format       string
	stdout       io.Writer
}

// testPolicyResult holds the aggregated test results.
type testPolicyResult struct {
	testDataValid  bool
	testDataErrors []string
	testsExecuted  bool
	results        *testResultDetail
}

// testResultDetail holds test execution counts.
type testResultDetail struct {
	Total  int      `json:"total"`
	Passed int      `json:"passed"`
	Failed int      `json:"failed"`
	Errors []string `json:"errors"`
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
		"Path to JSON test-data file for CUE schema pre-validation")
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

func runTestPolicy(ctx context.Context, params testPolicyRunParams) error {
	// Read policy file
	content, err := os.ReadFile(params.policyFile)
	if err != nil {
		return fmt.Errorf("reading policy file: %w", err)
	}

	result := testPolicyResult{
		testDataValid:  true,
		testDataErrors: make([]string, 0),
		testsExecuted:  false,
	}

	// Validate test data against CUE schema (when --test-data is provided)
	if params.testDataFile != "" {
		testDataBytes, err := os.ReadFile(params.testDataFile)
		if err != nil {
			return fmt.Errorf("reading test-data file: %w", err)
		}

		var testData map[string]interface{}
		if err := json.Unmarshal(testDataBytes, &testData); err != nil {
			return fmt.Errorf("parsing test-data JSON: %w", err)
		}

		// Load CUE schema for validation
		schemaRefs, err := buildSchemaRefs(params.platform, params.schemas)
		if err != nil {
			return err
		}

		_, cueSchemas, err := schema.LoadFromConfig(
			ctx, schemaRefs, schema.DefaultRegistry(),
		)
		if err != nil {
			return fmt.Errorf("loading schemas: %w", err)
		}

		cueSchema, ok := cueSchemas[params.platform]
		if !ok {
			return fmt.Errorf(
				"no schema found for platform %q; provide --schema %s=<source>",
				params.platform, params.platform,
			)
		}

		testDataErrs := validateTestData(testData, cueSchema)
		if len(testDataErrs) > 0 {
			result.testDataValid = false
			result.testDataErrors = testDataErrs
			return writeTestPolicyOutput(params.format, params.stdout, result)
		}
	}

	// Resolve evaluator
	evalRegistry := evaluator.DefaultRegistry()
	var eval evaluator.Evaluator
	if params.evalID != "" {
		eval, err = evalRegistry.Get(params.evalID)
		if err != nil {
			return fmt.Errorf("evaluator %q: %w", params.evalID, err)
		}
	} else {
		ids := evalRegistry.IDs()
		if len(ids) == 0 {
			return fmt.Errorf("no evaluators registered")
		}
		if len(ids) > 1 {
			return fmt.Errorf(
				"multiple evaluators available (%s); use --evaluator to select one",
				strings.Join(ids, ", "),
			)
		}
		eval, _ = evalRegistry.Get(ids[0])
	}

	filename := "policy" + eval.FileExtension()
	files := map[string]string{
		filename: string(content),
	}

	// Execute tests
	testResults, err := eval.Test(ctx, files)
	if err != nil {
		return fmt.Errorf("test execution failed: %w", err)
	}

	result.testsExecuted = true
	result.results = &testResultDetail{
		Total:  testResults.Total,
		Passed: testResults.Passed,
		Failed: testResults.Failed,
		Errors: testResults.Errors,
	}
	if result.results.Errors == nil {
		result.results.Errors = make([]string, 0)
	}

	return writeTestPolicyOutput(params.format, params.stdout, result)
}

// validateTestData validates test data against a CUE schema using unification.
func validateTestData(
	testData map[string]interface{}, cueSchema cue.Value,
) []string {
	cueCtx := cuecontext.New()
	dataVal := cueCtx.Encode(testData)
	if dataVal.Err() != nil {
		return []string{
			fmt.Sprintf("failed to encode test data: %v", dataVal.Err()),
		}
	}

	unified := cueSchema.Unify(dataVal)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return collectCUEValidationErrors(err)
	}

	return nil
}

// collectCUEValidationErrors extracts error messages from a CUE error.
func collectCUEValidationErrors(err error) []string {
	type errorList interface {
		Unwrap() []error
	}

	var errors []string
	if el, ok := err.(errorList); ok {
		for _, e := range el.Unwrap() {
			errors = append(errors, e.Error())
		}
	} else {
		errors = append(errors, err.Error())
	}
	return errors
}

func writeTestPolicyOutput(
	format string, w io.Writer, result testPolicyResult,
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
			"unknown format %q; valid formats: human, text, json",
			format,
		)
	}
}

func writeTestPolicyJSON(w io.Writer, result testPolicyResult) error {
	response := map[string]interface{}{
		"testDataValid":  result.testDataValid,
		"testDataErrors": result.testDataErrors,
		"testsExecuted":  result.testsExecuted,
	}
	if result.results != nil {
		response["results"] = map[string]interface{}{
			"total":  result.results.Total,
			"passed": result.results.Passed,
			"failed": result.results.Failed,
			"errors": result.results.Errors,
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(response)
}

func writeTestPolicyText(w io.Writer, result testPolicyResult) error {
	if !result.testDataValid {
		fmt.Fprintln(w, "[FAIL] Test data validation failed")
		for _, e := range result.testDataErrors {
			fmt.Fprintf(w, "  [TEST DATA ERROR] %s\n", e)
		}
		return nil
	}

	if !result.testsExecuted {
		fmt.Fprintln(w, "[SKIP] Tests were not executed")
		return nil
	}

	if result.results.Failed == 0 && len(result.results.Errors) == 0 {
		fmt.Fprintf(w, "[PASS] %d/%d tests passed\n",
			result.results.Passed, result.results.Total)
	} else {
		fmt.Fprintf(w, "[FAIL] %d/%d tests passed, %d failed\n",
			result.results.Passed, result.results.Total,
			result.results.Failed)
	}

	for _, e := range result.results.Errors {
		fmt.Fprintf(w, "  [ERROR] %s\n", e)
	}

	return nil
}

func writeTestPolicyHuman(w io.Writer, result testPolicyResult) error {
	if !result.testDataValid {
		fmt.Fprintln(w,
			styleFail.Render("✗ Test data validation failed"))
		fmt.Fprintln(w)
		fmt.Fprintln(w, styleTitle.Render("Test Data Errors"))
		for _, e := range result.testDataErrors {
			fmt.Fprintf(w, "  %s %s\n", styleFail.Render("✗"), e)
		}
		return nil
	}

	if !result.testsExecuted {
		fmt.Fprintln(w,
			styleDim.Render("Tests were not executed"))
		return nil
	}

	if result.results.Failed == 0 && len(result.results.Errors) == 0 {
		fmt.Fprintln(w, stylePass.Render(
			fmt.Sprintf("✓ %d/%d tests passed",
				result.results.Passed, result.results.Total)))
	} else {
		fmt.Fprintln(w, styleFail.Render(
			fmt.Sprintf("✗ %d/%d tests passed, %d failed",
				result.results.Passed, result.results.Total,
				result.results.Failed)))
	}

	if len(result.results.Errors) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, styleTitle.Render("Test Errors"))
		for _, e := range result.results.Errors {
			fmt.Fprintf(w, "  %s %s\n", styleFail.Render("✗"), e)
		}
	}

	return nil
}
