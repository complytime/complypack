// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/coverage"
	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/internal/schema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

// buildValidationResponse constructs the
// validate_policy MCP response from domain results.
func buildValidationResponse(
	r *evaluator.ValidatePolicyResult,
) (*mcp.CallToolResult, error) {
	violations := make(
		[]map[string]string, len(r.ContractViolations),
	)
	for i, v := range r.ContractViolations {
		violations[i] = map[string]string{
			"path":     v.Path,
			"location": v.Location,
		}
	}

	warnings := make(
		[]map[string]string, len(r.LintWarnings),
	)
	for i, w := range r.LintWarnings {
		warnings[i] = map[string]string{
			"rule":     w.Rule,
			"message":  w.Message,
			"location": w.Location,
		}
	}

	response := map[string]interface{}{
		"valid":              r.Valid,
		"syntaxErrors":       r.SyntaxErrors,
		"contractViolations": violations,
		"lintWarnings":       warnings,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to marshal response: %w", err,
		)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(responseJSON),
			},
		},
	}, nil
}

// buildTestPolicyResponse constructs the test_policy MCP response
// from domain results, with optional per-requirement attribution.
// perRequirement is optional: if non-nil, it is included in the response
// under the "perRequirement" key — even when empty. This lets consumers
// distinguish between "attribution was attempted but no tests matched the
// naming convention" (perRequirement: {}) and "feature not available"
// (perRequirement absent).
func buildTestPolicyResponse(
	r *evaluator.TestPolicyResult,
	perRequirement map[string]coverage.RequirementTestStatus,
) (*mcp.CallToolResult, error) {
	response := map[string]interface{}{
		"testDataValid":  r.TestDataValid,
		"testDataErrors": r.TestDataErrors,
		"testsExecuted":  r.TestsExecuted,
	}
	if r.Results != nil {
		resultsMap := map[string]interface{}{
			"total":  r.Results.Total,
			"passed": r.Results.Passed,
			"failed": r.Results.Failed,
			"errors": r.Results.Errors,
		}

		if perRequirement != nil {
			pr := make(map[string]string, len(perRequirement))
			for reqID, status := range perRequirement {
				pr[reqID] = string(status)
			}
			resultsMap["perRequirement"] = pr
		}

		response["results"] = resultsMap
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to marshal response: %w", err,
		)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(responseJSON),
			},
		},
	}, nil
}

// resolveEvaluator picks the evaluator from the
// store's registry using the shared Resolve function.
func resolveEvaluator(
	store *ResourceStore, id string,
) (evaluator.Evaluator, error) {
	if store.evaluators == nil {
		return nil, fmt.Errorf(
			"no evaluators available",
		)
	}
	return store.evaluators.Resolve(id)
}

// handleValidatePolicy handles the validate_policy
// MCP tool, delegating to evaluator.ValidatePolicy.
func handleValidatePolicy(
	store *ResourceStore,
) mcp.ToolHandler {
	return func(
		ctx context.Context,
		req *mcp.CallToolRequest,
	) (*mcp.CallToolResult, error) {
		var input struct {
			PolicyContent string `json:"policyContent"`
			Platform      string `json:"platform"`
			Evaluator     string `json:"evaluator"`
		}
		if err := json.Unmarshal(
			req.Params.Arguments, &input,
		); err != nil {
			return nil, fmt.Errorf(
				"failed to parse input: %w", err,
			)
		}

		eval, err := resolveEvaluator(
			store, input.Evaluator,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"evaluator not found: %w", err,
			)
		}

		cueSchema, err := store.CUESchema(
			input.Platform,
		)
		if err != nil {
			return nil, err
		}

		result := evaluator.ValidatePolicy(
			eval, input.PolicyContent, cueSchema,
		)
		return buildValidationResponse(result)
	}
}

// handleTestPolicy handles the test_policy MCP tool,
// delegating to schema.ValidateData and
// evaluator.TestPolicy.
func handleTestPolicy(
	store *ResourceStore,
) mcp.ToolHandler {
	return func(
		ctx context.Context,
		req *mcp.CallToolRequest,
	) (*mcp.CallToolResult, error) {
		var input struct {
			PolicyContent string                 `json:"policyContent"`
			TestData      map[string]interface{} `json:"testData"`
			Platform      string                 `json:"platform"`
			Evaluator     string                 `json:"evaluator"`
		}
		if err := json.Unmarshal(
			req.Params.Arguments, &input,
		); err != nil {
			return nil, fmt.Errorf(
				"failed to parse input: %w", err,
			)
		}

		// Validate test data against platform schema
		var testDataErrors []string
		cueSchema, err := store.CUESchema(
			input.Platform,
		)
		if err != nil {
			testDataErrors = []string{
				fmt.Sprintf(
					"unsupported platform %q: %v",
					input.Platform, err,
				),
			}
		} else {
			testDataErrors = schema.ValidateData(
				input.TestData, cueSchema,
			)
		}

		eval, err := resolveEvaluator(
			store, input.Evaluator,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"evaluator not found: %w", err,
			)
		}

		result, err := evaluator.TestPolicy(
			ctx, eval,
			input.PolicyContent, testDataErrors,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"test execution failed: %w", err,
			)
		}

		// Compute per-requirement test attribution.
		// Note: MCP receives policy content as a string (not from a directory),
		// so contentDir and mappingPath are empty — only convention-based mapping
		// applies. Override mapping files are supported via the CLI only.
		// Degrade gracefully on attribution failure — return test results
		// without per-requirement data rather than aborting the response.
		// This aligns with CLI behavior at pack.go:240 which logs a WARNING
		// and continues.
		var perReq map[string]coverage.RequirementTestStatus
		if result.Results != nil {
			var attrErr error
			perReq, attrErr = coverage.AttributeTests("", "", result.Results)
			if attrErr != nil {
				log.Printf("WARNING: test attribution failed: %v", attrErr)
			}
		}

		return buildTestPolicyResponse(result, perReq)
	}
}

// createValidateConfigTool creates the MCP tool definition for validate_config.
func createValidateConfigTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "validate_config",
		Description: "Validate a complypack.yaml configuration file against the JSON Schema, structural rules, and scope-specific requirements. Returns structured validation results.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the complypack.yaml configuration file to validate",
				},
				"unknownFields": map[string]interface{}{
					"type":        "string",
					"description": "How to handle unknown config fields: 'warn' (default) or 'error'",
					"enum":        toInterfaceSlice(config.ValidUnknownFields),
				},
				"scope": map[string]interface{}{
					"type":        "array",
					"description": "Validation scopes: pack, serve, init, or all (default: all)",
					"items": map[string]interface{}{
						"type": "string",
						"enum": toInterfaceSlice(config.ValidScopes),
					},
				},
			},
			"required": []interface{}{"path"},
		},
	}
}

// handleValidateConfig handles the validate_config MCP tool.
//
// Security note: the path parameter is passed directly to os.ReadFile via
// config.LoadConfig. This is safe because the MCP server uses stdio transport
// only (see cmd/complypack/cli/mcp.go), meaning the caller is a local process
// that already has equivalent filesystem access. If non-stdio transports are
// added in the future, path validation should be added here.
func handleValidateConfig() mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input struct {
			Path          string   `json:"path"`
			UnknownFields string   `json:"unknownFields"`
			Scope         []string `json:"scope"`
		}

		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}

		strict := input.UnknownFields == "error"

		var warnings bytes.Buffer
		cfg, err := config.LoadConfig(input.Path, strict, &warnings)
		if err != nil {
			response := map[string]interface{}{
				"valid":    false,
				"errors":   []string{err.Error()},
				"warnings": []string{},
				"scopes":   []interface{}{},
			}

			responseJSON, jsonErr := json.Marshal(response)
			if jsonErr != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", jsonErr)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: string(responseJSON),
					},
				},
			}, nil
		}

		// Run scope-specific validation
		scopeResults := cfg.ValidateScopes(input.Scope)

		scopeMaps := make([]map[string]interface{}, len(scopeResults))
		allValid := true
		for i, r := range scopeResults {
			scopeMaps[i] = map[string]interface{}{
				"scope": r.Scope,
				"valid": r.Valid,
				"error": r.Error,
			}
			if !r.Valid {
				allValid = false
			}
		}

		// Collect warnings as a string slice
		var warningList []string
		if warnings.Len() > 0 {
			warningList = append(warningList, warnings.String())
		}

		response := map[string]interface{}{
			"valid":    allValid,
			"errors":   []string{},
			"warnings": warningList,
			"scopes":   scopeMaps,
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
}

// toInterfaceSlice converts a string slice to an interface slice for JSON schema definitions.
func toInterfaceSlice(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// GetValidatePolicyHandler exposes handler for testing.
func GetValidatePolicyHandler(s *Server) mcp.ToolHandler {
	return handleValidatePolicy(s.ResourceStore)
}

// GetTestPolicyHandler exposes handler for testing.
func GetTestPolicyHandler(s *Server) mcp.ToolHandler {
	return handleTestPolicy(s.ResourceStore)
}
