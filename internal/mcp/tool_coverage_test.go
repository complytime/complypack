// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/complytime/complypack/internal/coverage"
	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/internal/requirement"
	"github.com/gemaraproj/go-gemara"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCoveragePolicy() *requirement.ResolvedPolicy {
	catalog := &gemara.ControlCatalog{
		Metadata: gemara.Metadata{Id: "coverage-catalog"},
		Controls: []gemara.Control{
			{
				Id: "CTL-001",
				AssessmentRequirements: []gemara.AssessmentRequirement{
					{Id: "CTL-001-AR1"},
					{Id: "CTL-001-AR2"},
				},
			},
		},
	}

	policy := &gemara.Policy{
		Metadata: gemara.Metadata{
			Id:                "coverage-policy",
			MappingReferences: []gemara.MappingReference{{Id: "coverage-catalog"}},
		},
		Imports: gemara.Imports{
			Catalogs: []gemara.CatalogImport{{ReferenceId: "coverage-catalog"}},
		},
		Adherence: gemara.Adherence{
			EvaluationMethods: []gemara.AcceptedMethod{
				{Id: "default-eval", Mode: gemara.ModeAutomated, Executor: gemara.Actor{Id: "opa"}},
			},
			AssessmentPlans: []gemara.AssessmentPlan{
				{Id: "ap-1", RequirementId: "CTL-001-AR1"},
				{Id: "ap-2", RequirementId: "CTL-001-AR2"},
			},
		},
	}

	set := &requirement.ArtifactSet{
		Catalogs: map[string]*gemara.ControlCatalog{"coverage-catalog": catalog},
		Policies: map[string]*gemara.Policy{"coverage-policy": policy},
		Guidance: make(map[string]*gemara.GuidanceCatalog),
		Mappings: make(map[string]*gemara.MappingDocument),
	}

	rp, err := requirement.ResolvePolicy(*policy, set)
	if err != nil {
		panic(err)
	}
	return rp
}

func TestHandleGetCoverageReport(t *testing.T) {
	// Set up a policy directory with a mapping file
	policyDir := t.TempDir()
	mappingData, err := json.Marshal(map[string]interface{}{
		"version": "1",
		"mappings": []map[string]string{
			{"id": "ctl_001_ar1", "requirement_id": "CTL-001-AR1"},
		},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(policyDir, "complytime-mapping.json"), mappingData, 0o600))

	evalRegistry := evaluator.DefaultRegistry()

	store := &ResourceStore{
		artifacts: map[string]any{},
		resolved: map[string]*requirement.ResolvedPolicy{
			"coverage-policy": testCoveragePolicy(),
		},
		schemas:    map[string][]byte{},
		evaluators: evalRegistry,
	}

	handler := handleGetCoverageReport(store)

	t.Run("valid input with partial coverage", func(t *testing.T) {
		input := map[string]interface{}{
			"policy":     "coverage-policy",
			"policy_dir": policyDir,
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

		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok)

		var report coverage.Report
		err = json.Unmarshal([]byte(textContent.Text), &report)
		require.NoError(t, err)

		assert.Equal(t, "coverage-policy", report.PolicyID)
		assert.Equal(t, 2, report.Metrics.TotalAutomated)
		assert.Equal(t, 1, report.Metrics.Implemented)
		assert.Equal(t, 1, report.Metrics.Gaps)
		assert.Equal(t, 50.0, report.Metrics.CoveragePercent)
	})

	t.Run("unknown policy name", func(t *testing.T) {
		input := map[string]interface{}{
			"policy":     "nonexistent",
			"policy_dir": policyDir,
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
				Arguments: json.RawMessage([]byte(`{invalid`)),
			},
		}

		result, err := handler(context.Background(), req)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestCreateGetCoverageReportTool(t *testing.T) {
	tool := createGetCoverageReportTool()

	assert.Equal(t, "get_coverage_report", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(map[string]interface{})
	require.True(t, ok)

	properties, ok := schema["properties"].(map[string]interface{})
	require.True(t, ok)

	_, ok = properties["policy"].(map[string]interface{})
	require.True(t, ok, "should have policy property")

	_, ok = properties["policy_dir"].(map[string]interface{})
	require.True(t, ok, "should have policy_dir property")

	_, ok = properties["evaluator"].(map[string]interface{})
	require.True(t, ok, "should have evaluator property")

	_, ok = properties["run_tests"].(map[string]interface{})
	require.True(t, ok, "should have run_tests property")

	required, ok := schema["required"].([]interface{})
	require.True(t, ok)
	assert.Contains(t, required, "policy")
	assert.Contains(t, required, "policy_dir")
}
