// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/evaluator"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// validateTestDataAgainstSchema validates test data against the compiled CUE
// schema. Uses CUE unification so validation works regardless of whether the
// schema was loaded from embedded JSON Schema, a CUE file, or a CUE module.
func validateTestDataAgainstSchema(testData map[string]interface{}, platform string, store *ResourceStore) []string {
	schema, err := store.CUESchema(platform)
	if err != nil {
		return []string{fmt.Sprintf("unsupported platform %q: %v", platform, err)}
	}

	cueCtx := cuecontext.New()
	dataVal := cueCtx.Encode(testData)
	if dataVal.Err() != nil {
		return []string{fmt.Sprintf("failed to encode test data: %v", dataVal.Err())}
	}

	unified := schema.Unify(dataVal)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return collectCUEErrors(err)
	}

	return nil
}

// collectCUEErrors extracts individual error messages from a CUE error,
// which may wrap multiple validation failures.
func collectCUEErrors(err error) []string {
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

// createValidatePolicyTool creates the MCP tool definition for validate_policy.
func createValidatePolicyTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "validate_policy",
		Description: "Validate policy syntax, contract compliance against platform schema, and linting. Read complypack://schema to discover available platforms. Read complypack://evaluator to discover available evaluators.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"policyContent": map[string]interface{}{
					"type":        "string",
					"description": "The policy source code to validate",
				},
				"platform": map[string]interface{}{
					"type":        "string",
					"description": "Target platform for contract validation. Read complypack://schema for available platforms.",
				},
				"evaluator": map[string]interface{}{
					"type":        "string",
					"description": "Evaluator ID (e.g., 'opa'). Omit to auto-select if only one is available. Read complypack://evaluator for the list.",
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
		Description: "Validate test data against platform schema, then execute policy tests. Read complypack://schema to discover available platforms. Read complypack://evaluator to discover available evaluators.",
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
					"description": "Target platform for test data validation. Read complypack://schema for available platforms.",
				},
				"evaluator": map[string]interface{}{
					"type":        "string",
					"description": "Evaluator ID (e.g., 'opa'). Omit to auto-select if only one is available. Read complypack://evaluator for the list.",
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
		"valid":              valid,
		"syntaxErrors":       syntaxErrStrs,
		"contractViolations": contractViolationMaps,
		"lintWarnings":       lintWarningMaps,
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
		"testDataValid": true,
		"testsExecuted": true,
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
			schema, err := store.CUESchema(input.Platform)
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

// createValidateConfigTool creates the MCP tool definition for validate_config.
func createValidateConfigTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "validate_config",
		Description: "Validate a complypack.yaml configuration file against the JSON Schema and structural rules. Returns validation errors and warnings.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the complypack.yaml configuration file to validate",
				},
				"strict": map[string]interface{}{
					"type":        "boolean",
					"description": "Treat unknown config fields as errors (default: false)",
				},
			},
			"required": []interface{}{"path"},
		},
	}
}

// handleValidateConfig handles the validate_config MCP tool.
func handleValidateConfig() mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input struct {
			Path   string `json:"path"`
			Strict bool   `json:"strict"`
		}

		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}

		var warnings bytes.Buffer
		_, err := config.LoadConfig(input.Path, input.Strict, &warnings)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("validation failed: %v", err),
					},
				},
				IsError: true,
			}, nil
		}

		msg := fmt.Sprintf("%s is valid", input.Path)
		if warnings.Len() > 0 {
			msg += "\n" + warnings.String()
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: msg,
				},
			},
		}, nil
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

// GetValidateConfigHandler exposes handler for testing.
func GetValidateConfigHandler() mcp.ToolHandler {
	return handleValidateConfig()
}
