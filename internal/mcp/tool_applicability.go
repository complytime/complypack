// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/complytime/complypack/internal/requirement"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func createGetApplicabilityGroupsTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_applicability_groups",
		Description: "Get applicability group definitions and their requirement memberships from a catalog or policy. Returns group metadata (id, title, description) and which requirements belong to each group.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"catalogName": map[string]interface{}{
					"type":        "string",
					"description": "Name of the catalog or policy to inspect",
				},
				"requirementIds": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
					"description": "Optional: filter to groups containing these requirement IDs",
				},
			},
			"required": []interface{}{"catalogName"},
		},
	}
}

func handleGetApplicabilityGroups(store *ResourceStore) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input struct {
			CatalogName    string   `json:"catalogName"`
			RequirementIDs []string `json:"requirementIds"`
		}

		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}

		rp, err := findResolvedPolicy(store, input.CatalogName)
		if err != nil {
			return nil, err
		}

		result := requirement.CollectApplicabilityGroups(rp, input.RequirementIDs)

		responseData, err := json.Marshal(struct {
			Catalog string `json:"catalog"`
			*requirement.ApplicabilityGroupResult
		}{
			Catalog:                  input.CatalogName,
			ApplicabilityGroupResult: result,
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

// GetApplicabilityGroupsHandler returns the handler (for testing).
func GetApplicabilityGroupsHandler(store *ResourceStore) mcp.ToolHandler {
	return handleGetApplicabilityGroups(store)
}
