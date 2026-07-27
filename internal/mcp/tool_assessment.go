// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/complytime/complypack/internal/requirement"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// createGetAssessmentRequirementsTool creates the MCP tool definition.
func createGetAssessmentRequirementsTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_assessment_requirements",
		Description: "Extract assessment requirements from a policy or catalog with structured parameters from assessment plans",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"catalogName": map[string]interface{}{
					"type":        "string",
					"description": "Name of the catalog or policy to extract from (e.g., 'my-policy')",
				},
				"controlId": map[string]interface{}{
					"type":        "string",
					"description": "Optional: Specific control ID to filter requirements (e.g., 'CTRL-001')",
				},
				"scope": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
					"description": "Optional: Filter requirements by applicability groups (e.g., ['maturity-1', 'maturity-2']). Returns requirements whose applicability contains any of the given values.",
				},
			},
			"required": []interface{}{"catalogName"},
		},
	}
}

// handleGetAssessmentRequirements extracts assessment requirements
// from a policy or catalog.
func handleGetAssessmentRequirements(store *ResourceStore) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse input
		var input struct {
			CatalogName string   `json:"catalogName"`
			ControlID   string   `json:"controlId"`
			Scope       []string `json:"scope"`
		}

		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}

		rp, err := findResolvedPolicy(store, input.CatalogName)
		if err != nil {
			return nil, err
		}

		requirements := requirement.ExtractAssessmentRequirements(
			rp, input.ControlID, input.Scope,
		)

		// Build response
		responseData, err := json.Marshal(map[string]interface{}{
			"catalog":      input.CatalogName,
			"control_id":   input.ControlID,
			"scope":        input.Scope,
			"count":        len(requirements),
			"requirements": requirements,
		})
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

// GetAssessmentRequirementsHandler returns the handler (for testing).
func GetAssessmentRequirementsHandler(store *ResourceStore) mcp.ToolHandler {
	return handleGetAssessmentRequirements(store)
}
