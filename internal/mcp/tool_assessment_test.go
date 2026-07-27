// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/complytime/complypack/internal/requirement"
	"github.com/gemaraproj/go-gemara"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testResolvedPolicy() *requirement.ResolvedPolicy {
	catalog := &gemara.ControlCatalog{
		Metadata: gemara.Metadata{Id: "test-catalog"},
		Controls: []gemara.Control{
			{
				Id: "TEST-001",
				AssessmentRequirements: []gemara.AssessmentRequirement{
					{
						Id:            "TEST-001-AR1",
						Text:          "Test requirement",
						Applicability: []string{"maturity-1", "maturity-2", "maturity-3"},
					},
					{
						Id:            "TEST-001-AR2",
						Text:          "Second requirement",
						Applicability: []string{"maturity-2", "maturity-3"},
					},
				},
			},
			{
				Id: "TEST-002",
				AssessmentRequirements: []gemara.AssessmentRequirement{
					{
						Id:            "TEST-002-AR1",
						Text:          "Third requirement",
						Applicability: []string{"maturity-3"},
					},
				},
			},
		},
	}

	policy := &gemara.Policy{
		Metadata: gemara.Metadata{
			Id: "test-policy",
			MappingReferences: []gemara.MappingReference{
				{Id: "test-catalog"},
			},
		},
		Imports: gemara.Imports{
			Catalogs: []gemara.CatalogImport{
				{ReferenceId: "test-catalog"},
			},
		},
		Adherence: gemara.Adherence{
			AssessmentPlans: []gemara.AssessmentPlan{
				{
					RequirementId: "TEST-001-AR1",
					Parameters: []gemara.Parameter{
						{Label: "threshold", AcceptedValues: []string{"90"}},
					},
				},
			},
		},
	}

	set := &requirement.ArtifactSet{
		Catalogs: map[string]*gemara.ControlCatalog{"test-catalog": catalog},
		Policies: map[string]*gemara.Policy{"test-policy": policy},
		Guidance: make(map[string]*gemara.GuidanceCatalog),
	}

	rp, err := requirement.ResolvePolicy(*policy, set)
	if err != nil {
		panic(err)
	}
	return rp
}

func TestHandleGetAssessmentRequirements(t *testing.T) {
	store := &ResourceStore{
		artifacts: map[string]any{},
		resolved: map[string]*requirement.ResolvedPolicy{
			"test-policy": testResolvedPolicy(),
		},
		schemas: map[string][]byte{},
	}

	handler := handleGetAssessmentRequirements(store)

	t.Run("successful extraction", func(t *testing.T) {
		input := map[string]interface{}{
			"catalogName": "test-policy",
		}
		inputJSON, err := json.Marshal(input)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: json.RawMessage(inputJSON),
			},
		}

		result, err := handler(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Content, 1)

		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok)

		var response map[string]interface{}
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		assert.Equal(t, "test-policy", response["catalog"])
		assert.Equal(t, float64(3), response["count"])

		requirements, ok := response["requirements"].([]interface{})
		require.True(t, ok)
		assert.Len(t, requirements, 3)
	})

	t.Run("catalog name fallback", func(t *testing.T) {
		catalog := &gemara.ControlCatalog{
			Metadata: gemara.Metadata{Id: "bare-catalog"},
			Controls: []gemara.Control{
				{
					Id: "CAT-001",
					AssessmentRequirements: []gemara.AssessmentRequirement{
						{Id: "CAT-001-AR1", Text: "Catalog requirement", Applicability: []string{"maturity-1"}},
					},
				},
			},
		}
		catalogStore := &ResourceStore{
			artifacts: map[string]any{"bare-catalog": catalog},
			resolved:  map[string]*requirement.ResolvedPolicy{},
			schemas:   map[string][]byte{},
		}
		catalogHandler := handleGetAssessmentRequirements(catalogStore)

		input := map[string]interface{}{
			"catalogName": "bare-catalog",
		}
		inputJSON, err := json.Marshal(input)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: json.RawMessage(inputJSON),
			},
		}

		result, err := catalogHandler(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok)

		var response map[string]interface{}
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(1), response["count"])
		requirements := response["requirements"].([]interface{})
		firstReq := requirements[0].(map[string]interface{})
		assert.Equal(t, "CAT-001-AR1", firstReq["id"])
	})

	t.Run("policy not found", func(t *testing.T) {
		input := map[string]interface{}{
			"catalogName": "nonexistent",
		}
		inputJSON, err := json.Marshal(input)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: json.RawMessage(inputJSON),
			},
		}

		result, err := handler(context.Background(), req)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("invalid input", func(t *testing.T) {
		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: json.RawMessage([]byte(`{invalid json`)),
			},
		}

		result, err := handler(context.Background(), req)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid input")
	})

	t.Run("filter by control ID", func(t *testing.T) {
		input := map[string]interface{}{
			"catalogName": "test-policy",
			"controlId":   "TEST-001",
		}
		inputJSON, err := json.Marshal(input)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: json.RawMessage(inputJSON),
			},
		}

		result, err := handler(context.Background(), req)
		require.NoError(t, err)

		textContent := result.Content[0].(*mcp.TextContent)
		var response map[string]interface{}
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		assert.Equal(t, "TEST-001", response["control_id"])
		assert.Equal(t, float64(2), response["count"])
	})

	t.Run("parameters from assessment plans", func(t *testing.T) {
		input := map[string]interface{}{
			"catalogName": "test-policy",
			"controlId":   "TEST-001",
		}
		inputJSON, err := json.Marshal(input)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: json.RawMessage(inputJSON),
			},
		}

		result, err := handler(context.Background(), req)
		require.NoError(t, err)

		textContent := result.Content[0].(*mcp.TextContent)
		var response map[string]interface{}
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		requirements := response["requirements"].([]interface{})
		firstReq := requirements[0].(map[string]interface{})
		params := firstReq["parameters"].(map[string]interface{})
		assert.Equal(t, "90", params["threshold"])
	})

	t.Run("filter by scope", func(t *testing.T) {
		input := map[string]interface{}{
			"catalogName": "test-policy",
			"scope":       []string{"maturity-2"},
		}
		inputJSON, err := json.Marshal(input)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: json.RawMessage(inputJSON),
			},
		}

		result, err := handler(context.Background(), req)
		require.NoError(t, err)

		textContent := result.Content[0].(*mcp.TextContent)
		var response map[string]interface{}
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(2), response["count"])
	})
}

func TestCreateGetAssessmentRequirementsTool(t *testing.T) {
	tool := createGetAssessmentRequirementsTool()

	assert.Equal(t, "get_assessment_requirements", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(map[string]interface{})
	require.True(t, ok)

	assert.Equal(t, "object", schema["type"])

	properties, ok := schema["properties"].(map[string]interface{})
	require.True(t, ok)

	catalogName, ok := properties["catalogName"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "string", catalogName["type"])

	controlId, ok := properties["controlId"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "string", controlId["type"])

	scope, ok := properties["scope"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "array", scope["type"])

	required, ok := schema["required"].([]interface{})
	require.True(t, ok)
	assert.Contains(t, required, "catalogName")
}
