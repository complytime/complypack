// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/schemas"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// validateTestDataAgainstSchema validates test data against JSON Schema.
func validateTestDataAgainstSchema(testData map[string]interface{}, platform string, store *ResourceStore) []string {
	// Validate platform
	uri := fmt.Sprintf("%s://%s/%s", URIScheme, ResourceTypeSchema, platform)

	// Get schema bytes from store
	ctx := context.Background()
	contents, err := store.ReadResource(ctx, uri)
	if err != nil {
		return []string{fmt.Sprintf("unsupported platform %q (available: %v)", platform, schemas.BuiltInPlatforms)}
	}

	if len(contents) == 0 {
		return []string{"schema not found"}
	}

	// Compile schema
	compiler := jsonschema.NewCompiler()
	schemaURL := fmt.Sprintf("schema://%s", platform)
	schemaReader := bytes.NewReader([]byte(contents[0].Text))
	if err := compiler.AddResource(schemaURL, schemaReader); err != nil {
		return []string{fmt.Sprintf("failed to compile schema: %v", err)}
	}

	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		return []string{fmt.Sprintf("failed to compile schema: %v", err)}
	}

	// Validate test data
	if err := schema.Validate(testData); err != nil {
		var validationErrors []string
		if valErr, ok := err.(*jsonschema.ValidationError); ok {
			validationErrors = collectValidationErrors(valErr)
		} else {
			validationErrors = []string{err.Error()}
		}
		return validationErrors
	}

	return nil
}

// collectValidationErrors recursively collects validation error messages.
func collectValidationErrors(err *jsonschema.ValidationError) []string {
	var errors []string

	// Add this error's message
	if err.Message != "" {
		errors = append(errors, fmt.Sprintf("%s: %s", err.InstanceLocation, err.Message))
	}

	// Recursively collect from causes
	for _, cause := range err.Causes {
		errors = append(errors, collectValidationErrors(cause)...)
	}

	return errors
}

// createValidatePolicyTool creates the MCP tool definition for validate_policy.
func createValidatePolicyTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "validate_policy",
		Description: "Validate policy syntax, contract compliance against platform schema, and linting. Read complypack://evaluator to discover available evaluators.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"policyContent": map[string]interface{}{
					"type":        "string",
					"description": "The policy source code to validate",
				},
				"platform": map[string]interface{}{
					"type":        "string",
					"description": "Target platform for contract validation",
				},
				"evaluator": map[string]interface{}{
					"type":        "string",
					"description": "Evaluator ID (e.g., 'io.complytime.opa'). Omit to auto-select if only one is available. Read complypack://evaluator for the list.",
				},
			},
			"required": []interface{}{"policyContent", "platform"},
		},
	}
}

// createTestPolicyTool creates the MCP tool definition for test_policy.
func createTestPolicyTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "test_policy",
		Description: "Validate test data against platform schema, then execute policy tests. Read complypack://evaluator to discover available evaluators.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"policyContent": map[string]interface{}{
					"type":        "string",
					"description": "The policy source code to test",
				},
				"testData": map[string]interface{}{
					"type":        "object",
					"description": "Test data conforming to platform schema (e.g., Kubernetes manifest)",
				},
				"platform": map[string]interface{}{
					"type":        "string",
					"description": "Target platform for test data validation",
				},
				"evaluator": map[string]interface{}{
					"type":        "string",
					"description": "Evaluator ID (e.g., 'io.complytime.opa'). Omit to auto-select if only one is available. Read complypack://evaluator for the list.",
				},
			},
			"required": []interface{}{"policyContent", "testData", "platform"},
		},
	}
}

// buildValidationResponse constructs the validate_policy response.
func buildValidationResponse(valid bool, syntaxErrs []error, violations []evaluator.ContractViolation, warnings []evaluator.LintWarning) (*mcp.CallToolResult, error) {
	// Convert syntax errors to strings
	syntaxErrStrs := make([]string, len(syntaxErrs))
	for i, err := range syntaxErrs {
		syntaxErrStrs[i] = err.Error()
	}

	// Convert contract violations to response format
	contractViolationMaps := make([]map[string]string, len(violations))
	for i, v := range violations {
		contractViolationMaps[i] = map[string]string{
			"path":     v.Path,
			"location": v.Location,
		}
	}

	// Convert lint warnings to response format
	lintWarningMaps := make([]map[string]string, len(warnings))
	for i, w := range warnings {
		lintWarningMaps[i] = map[string]string{
			"rule":     w.Rule,
			"message":  w.Message,
			"location": w.Location,
		}
	}

	// Build response
	response := map[string]interface{}{
		"valid":               valid,
		"syntaxErrors":        syntaxErrStrs,
		"contractViolations":  contractViolationMaps,
		"lintWarnings":        lintWarningMaps,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(responseJSON),
			},
		},
	}, nil
}

// buildTestDataErrorResponse constructs response for invalid test data.
func buildTestDataErrorResponse(errors []string) (*mcp.CallToolResult, error) {
	response := map[string]interface{}{
		"testDataValid":  false,
		"testDataErrors": errors,
		"testsExecuted":  false,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(responseJSON),
			},
		},
	}, nil
}

// buildTestResultsResponse constructs the test_policy response.
func buildTestResultsResponse(results *evaluator.TestResults) (*mcp.CallToolResult, error) {
	response := map[string]interface{}{
		"testDataValid":  true,
		"testsExecuted":  true,
		"results": map[string]interface{}{
			"total":  results.Total,
			"passed": results.Passed,
			"failed": results.Failed,
			"errors": results.Errors,
		},
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(responseJSON),
			},
		},
	}, nil
}

// resolveEvaluator picks the evaluator from the store's registry.
// If id is provided, looks it up directly. If empty and only one evaluator
// is registered, auto-selects it. Otherwise returns an error listing options.
func resolveEvaluator(store *ResourceStore, id string) (evaluator.Evaluator, error) {
	if store.evaluators == nil {
		return nil, fmt.Errorf("no evaluators available")
	}

	if id != "" {
		return store.evaluators.Get(id)
	}

	ids := store.evaluators.IDs()
	if len(ids) == 0 {
		return nil, fmt.Errorf("no evaluators registered")
	}
	if len(ids) == 1 {
		return store.evaluators.Get(ids[0])
	}
	return nil, fmt.Errorf("multiple evaluators available, specify one: %v", ids)
}

// handleValidatePolicy handles the validate_policy MCP tool.
func handleValidatePolicy(store *ResourceStore) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse input
		var input struct {
			PolicyContent string `json:"policyContent"`
			Platform      string `json:"platform"`
			Evaluator     string `json:"evaluator"`
		}

		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}

		eval, err := resolveEvaluator(store, input.Evaluator)
		if err != nil {
			return nil, fmt.Errorf("evaluator not found: %w", err)
		}

		filename := "policy" + eval.FileExtension()

		// Validate syntax
		syntaxErrs := eval.Validate(filename, input.PolicyContent)

		// Load CUE schema and check contract (only if syntax is valid)
		var contractViolations []evaluator.ContractViolation
		var lintWarnings []evaluator.LintWarning

		if len(syntaxErrs) == 0 {
			schema, err := loadCUESchemaForPlatform(input.Platform)
			if err != nil {
				return nil, err
			}

			contractViolations, err = eval.CheckContract(filename, input.PolicyContent, schema)
			if err != nil {
				return nil, fmt.Errorf("contract check failed: %w", err)
			}

			// Run lint (graceful degradation if regal not available)
			lintWarnings, _ = eval.Lint(filename, input.PolicyContent)
		}

		// Build response
		valid := len(syntaxErrs) == 0 && len(contractViolations) == 0
		return buildValidationResponse(valid, syntaxErrs, contractViolations, lintWarnings)
	}
}

// handleTestPolicy handles the test_policy MCP tool.
func handleTestPolicy(store *ResourceStore) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse input
		var input struct {
			PolicyContent string                 `json:"policyContent"`
			TestData      map[string]interface{} `json:"testData"`
			Platform      string                 `json:"platform"`
			Evaluator     string                 `json:"evaluator"`
		}

		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}

		// Validate test data against platform schema
		testDataErrs := validateTestDataAgainstSchema(input.TestData, input.Platform, store)
		if len(testDataErrs) > 0 {
			return buildTestDataErrorResponse(testDataErrs)
		}

		eval, err := resolveEvaluator(store, input.Evaluator)
		if err != nil {
			return nil, fmt.Errorf("evaluator not found: %w", err)
		}

		filename := "policy" + eval.FileExtension()

		// Construct test files
		files := map[string]string{
			filename: input.PolicyContent,
		}

		// Execute tests
		results, err := eval.Test(ctx, files)
		if err != nil {
			return nil, fmt.Errorf("test execution failed: %w", err)
		}

		// Build response
		return buildTestResultsResponse(results)
	}
}

// GetValidatePolicyHandler exposes handler for testing.
func GetValidatePolicyHandler(s *Server) mcp.ToolHandler {
	return handleValidatePolicy(s.ResourceStore)
}

// GetTestPolicyHandler exposes handler for testing.
func GetTestPolicyHandler(s *Server) mcp.ToolHandler {
	return handleTestPolicy(s.ResourceStore)
}
