// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/complytime/complypack/internal/requirement"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func createGetAutomationTriageTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_automation_triage",
		Description: "Classify a policy's assessment plans as Automated or Manual based on evaluation methods. Returns the automation split with executor details, eliminating the need to parse adherence YAML manually.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"policyName": map[string]interface{}{
					"type":        "string",
					"description": "Name of the resolved policy to triage",
				},
			},
			"required": []interface{}{"policyName"},
		},
	}
}

func handleGetAutomationTriage(store *ResourceStore) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input struct {
			PolicyName string `json:"policyName"`
		}

		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}

		rp, err := findResolvedPolicy(store, input.PolicyName)
		if err != nil {
			return nil, err
		}

		result := requirement.TriageAssessmentPlans(rp)

		responseData, err := json.Marshal(result)
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
