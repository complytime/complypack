// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/complytime/complypack/internal/requirement"
	"github.com/gemaraproj/go-gemara"
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

// AssessmentRequirementInfo contains assessment requirement data with parameters.
type AssessmentRequirementInfo struct {
	ID            string            `json:"id"`
	ControlID     string            `json:"control_id"`
	Text          string            `json:"text"`
	Applicability []string          `json:"applicability,omitempty"`
	Parameters    map[string]string `json:"parameters,omitempty"`
}

// handleGetAssessmentRequirements extracts assessment requirements from a policy or catalog.
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

		rp, found := store.resolved[input.CatalogName]
		if !found {
			rp, found = resolveFromCatalog(store, input.CatalogName)
			if !found {
				return nil, fmt.Errorf("policy or catalog %q not found", input.CatalogName)
			}
		}
		requirements := extractFromResolvedPolicy(rp, input.ControlID, input.Scope)

		// Build control-level summaries
		controlIDs := rp.ControlIDs()
		if input.ControlID != "" {
			controlIDs = []string{input.ControlID}
		}
		summary, controls := rp.ControlSummaries(controlIDs, input.Scope)

		// Build response
		responseData, err := json.Marshal(map[string]interface{}{
			"catalog":      input.CatalogName,
			"control_id":   input.ControlID,
			"scope":        input.Scope,
			"count":        len(requirements),
			"summary":      summary,
			"controls":     controls,
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

// resolveFromCatalog wraps a bare catalog in a synthetic ResolvedPolicy so
// get_assessment_requirements works with catalog names, not just policy names.
func resolveFromCatalog(store *ResourceStore, name string) (*requirement.ResolvedPolicy, bool) {
	art, ok := store.artifacts[name]
	if !ok {
		return nil, false
	}
	cat, ok := art.(*gemara.ControlCatalog)
	if !ok {
		return nil, false
	}
	set := &requirement.ArtifactSet{
		Catalogs: map[string]*gemara.ControlCatalog{name: cat},
		Policies: make(map[string]*gemara.Policy),
		Guidance: make(map[string]*gemara.GuidanceCatalog),
		Mappings: make(map[string]*gemara.MappingDocument),
	}
	syntheticPolicy := gemara.Policy{
		Metadata: gemara.Metadata{
			Id:                name + "-synthetic",
			MappingReferences: []gemara.MappingReference{{Id: name}},
		},
		Imports: gemara.Imports{
			Catalogs: []gemara.CatalogImport{{ReferenceId: name}},
		},
	}
	rp, err := requirement.ResolvePolicy(syntheticPolicy, set)
	if err != nil {
		return nil, false
	}
	return rp, true
}

// extractFromResolvedPolicy extracts requirements from a resolved policy graph.
func extractFromResolvedPolicy(rp *requirement.ResolvedPolicy, filterControlID string, filterScope []string) []AssessmentRequirementInfo {
	var results []AssessmentRequirementInfo

	controlIDs := rp.ControlIDs()
	if filterControlID != "" {
		controlIDs = []string{filterControlID}
	}

	for _, controlID := range controlIDs {
		for _, req := range rp.RequirementsForControl(controlID) {
			if len(filterScope) > 0 && !requirement.ApplicabilityIntersects(req.Applicability, filterScope) {
				continue
			}
			info := AssessmentRequirementInfo{
				ID:            req.Id,
				ControlID:     controlID,
				Text:          req.Text,
				Applicability: req.Applicability,
				Parameters:    make(map[string]string),
			}

			for _, param := range rp.ParametersForRequirement(req.Id) {
				if len(param.AcceptedValues) == 1 {
					info.Parameters[param.Label] = param.AcceptedValues[0]
				} else if len(param.AcceptedValues) > 1 {
					info.Parameters[param.Label] = strings.Join(param.AcceptedValues, ", ")
				}
				if param.Description != "" {
					info.Parameters[param.Label+"_description"] = param.Description
				}
			}

			results = append(results, info)
		}
	}

	return results
}

// GetAssessmentRequirementsHandler returns the handler (for testing).
func GetAssessmentRequirementsHandler(store *ResourceStore) mcp.ToolHandler {
	return handleGetAssessmentRequirements(store)
}
