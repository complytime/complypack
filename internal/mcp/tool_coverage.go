// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/complytime/complypack/internal/coverage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func createGetCoverageReportTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "get_coverage_report",
		Description: "Generate a coverage report comparing a policy's in-scope assessment " +
			"requirements against enforcement artifacts in a directory. Returns per-requirement " +
			"status (implemented, gap) and aggregate coverage metrics.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"policy": map[string]interface{}{
					"type":        "string",
					"description": "Name of the resolved policy to check coverage for",
				},
				"policy_dir": map[string]interface{}{
					"type":        "string",
					"description": "Path to the directory containing enforcement artifacts (e.g., .rego files)",
				},
				"evaluator": map[string]interface{}{
					"type":        "string",
					"description": "Evaluator ID (e.g., 'opa'). Auto-detected if omitted.",
				},
				"run_tests": map[string]interface{}{
					"type":        "boolean",
					"description": "Execute tests for pass/fail enrichment. Defaults to false.",
				},
			},
			"required": []interface{}{"policy", "policy_dir"},
		},
	}
}

func handleGetCoverageReport(store *ResourceStore) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input struct {
			Policy    string `json:"policy"`
			PolicyDir string `json:"policy_dir"`
			Evaluator string `json:"evaluator"`
			RunTests  bool   `json:"run_tests"`
		}

		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}

		// Look up resolved policy
		rp, found := store.resolved[input.Policy]
		if !found {
			rp, found = resolveFromCatalog(store, input.Policy)
			if !found {
				return nil, fmt.Errorf("policy %q not found", input.Policy)
			}
		}

		// Resolve evaluator
		eval, err := resolveEvaluator(store, input.Evaluator)
		if err != nil {
			return nil, fmt.Errorf("evaluator resolution failed: %w", err)
		}

		// Run coverage engine
		report, err := coverage.Run(ctx, coverage.Options{
			ResolvedPolicy: rp,
			PolicyDir:      input.PolicyDir,
			Evaluator:      eval,
			RunTests:       input.RunTests,
		})
		if err != nil {
			return nil, fmt.Errorf("coverage analysis failed: %w", err)
		}

		responseData, err := json.Marshal(report)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(responseData),
				},
			},
		}, nil
	}
}
