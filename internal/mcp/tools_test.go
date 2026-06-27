// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"cuelang.org/go/cue"
	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/internal/schema"
	"github.com/complytime/complypack/schemas"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLoadAllSchemas loads a representative subset of schemas for testing.
func testLoadAllSchemas(t *testing.T) (map[string][]byte, map[string]cue.Value) {
	t.Helper()
	ctx := context.Background()

	refs := []config.SchemaRef{
		{Platform: "kubernetes-deployment"},
	}

	schemaMap, cueSchemaMap, err := loadSchemas(ctx, refs, schema.DefaultRegistry())
	require.NoError(t, err)
	return schemaMap, cueSchemaMap
}

func TestLoadSchemaFromIndex(t *testing.T) {
	ctx := context.Background()
	reg := schema.DefaultRegistry()

	index, err := schemas.LoadIndex()
	require.NoError(t, err)

	tests := []struct {
		name        string
		platform    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid kubernetes-deployment platform",
			platform: "kubernetes-deployment",
			wantErr:  false,
		},
		{
			name:     "valid ci-github-actions platform",
			platform: "ci-github-actions",
			wantErr:  false,
		},
		{
			name:        "unknown platform",
			platform:    "unknown",
			wantErr:     true,
			errContains: "no loader matched",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := ""
			if entry, ok := index[tt.platform]; ok {
				source = entry.Source
			}
			s, err := reg.Load(ctx, source, tt.platform)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.True(t, s.CUE.Exists(), "CUE schema should exist")
			}
		})
	}
}

func TestCreateValidatePolicyTool(t *testing.T) {
	tool := createValidatePolicyTool()

	assert.Equal(t, "validate_policy", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, tool.InputSchema)

	// Verify input schema has required fields
	schema := tool.InputSchema.(map[string]interface{})
	props := schema["properties"].(map[string]interface{})

	assert.Contains(t, props, "policyContent")
	assert.Contains(t, props, "platform")

	required := schema["required"].([]interface{})
	assert.Contains(t, required, "policyContent")
	assert.Contains(t, required, "platform")
}

func TestCreateTestPolicyTool(t *testing.T) {
	tool := createTestPolicyTool()

	assert.Equal(t, "test_policy", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, tool.InputSchema)

	// Verify input schema has required fields
	schema := tool.InputSchema.(map[string]interface{})
	props := schema["properties"].(map[string]interface{})

	assert.Contains(t, props, "policyContent")
	assert.Contains(t, props, "testData")
	assert.Contains(t, props, "platform")

	required := schema["required"].([]interface{})
	assert.Contains(t, required, "policyContent")
	assert.Contains(t, required, "testData")
	assert.Contains(t, required, "platform")
}

func TestValidateTestDataAgainstSchema(t *testing.T) {
	// Create resource store with schemas
	schemaMap, cueSchemaMap := testLoadAllSchemas(t)
	store := NewResourceStore(map[string]any{}, nil, schemaMap, cueSchemaMap, evaluator.DefaultRegistry())

	tests := []struct {
		name        string
		testData    map[string]interface{}
		platform    string
		wantErrors  bool
		errContains string
	}{
		{
			name: "valid kubernetes deployment",
			testData: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name": "test-deployment",
				},
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "test",
						},
					},
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"app": "test",
							},
						},
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "test",
									"image": "nginx:latest",
								},
							},
						},
					},
				},
			},
			platform:   "kubernetes-deployment",
			wantErrors: false,
		},
		{
			name:        "unknown platform",
			testData:    map[string]interface{}{},
			platform:    "unknown",
			wantErrors:  true,
			errContains: "unsupported platform",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateTestDataAgainstSchema(tt.testData, tt.platform, store)
			if tt.wantErrors {
				assert.NotEmpty(t, errs, "expected validation errors")
				if tt.errContains != "" {
					found := false
					for _, e := range errs {
						if assert.Contains(t, e, tt.errContains) {
							found = true
							break
						}
					}
					assert.True(t, found, "expected error containing %q", tt.errContains)
				}
			} else {
				assert.Empty(t, errs, "expected no validation errors")
			}
		})
	}
}

func TestBuildValidationResponse(t *testing.T) {
	tests := []struct {
		name       string
		valid      bool
		syntaxErrs []error
		violations []evaluator.ContractViolation
		warnings   []evaluator.LintWarning
		wantValid  bool
	}{
		{
			name:       "valid policy",
			valid:      true,
			syntaxErrs: nil,
			violations: nil,
			warnings:   nil,
			wantValid:  true,
		},
		{
			name:       "syntax errors",
			valid:      false,
			syntaxErrs: []error{fmt.Errorf("syntax error at line 5")},
			violations: nil,
			warnings:   nil,
			wantValid:  false,
		},
		{
			name:       "contract violations",
			valid:      false,
			syntaxErrs: nil,
			violations: []evaluator.ContractViolation{
				{Path: "input.invalid.field", Location: "policy.rego:10:5"},
			},
			warnings:  nil,
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildValidationResponse(tt.valid, tt.syntaxErrs, tt.violations, tt.warnings)
			require.NoError(t, err)

			// Parse result content
			textContent, ok := result.Content[0].(*mcp.TextContent)
			require.True(t, ok, "expected TextContent")

			var response map[string]interface{}
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.wantValid, response["valid"])
		})
	}
}

func TestBuildTestDataErrorResponse(t *testing.T) {
	errors := []string{
		"input.kind: invalid value",
		"input.metadata.name: required",
	}

	result, err := buildTestDataErrorResponse(errors)
	require.NoError(t, err)

	// Parse result content
	var response map[string]interface{}
	err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &response)
	require.NoError(t, err)

	assert.False(t, response["testDataValid"].(bool))
	assert.False(t, response["testsExecuted"].(bool))
	testDataErrs := response["testDataErrors"].([]interface{})
	assert.Len(t, testDataErrs, 2)
}

func TestBuildTestResultsResponse(t *testing.T) {
	results := &evaluator.TestResults{
		Total:  5,
		Passed: 3,
		Failed: 2,
		Errors: []string{
			"test_deny_root: expected denial",
			"test_labels: assertion failed",
		},
	}

	result, err := buildTestResultsResponse(results)
	require.NoError(t, err)

	// Parse result content
	var response map[string]interface{}
	err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &response)
	require.NoError(t, err)

	assert.True(t, response["testDataValid"].(bool))
	assert.True(t, response["testsExecuted"].(bool))

	testResults := response["results"].(map[string]interface{})
	assert.Equal(t, float64(5), testResults["total"])
	assert.Equal(t, float64(3), testResults["passed"])
	assert.Equal(t, float64(2), testResults["failed"])
}

func TestHandleValidatePolicy(t *testing.T) {
	// Create resource store with both kubernetes and ci schemas
	ctx := context.Background()
	refs := []config.SchemaRef{
		{Platform: "kubernetes-deployment"},
		{Platform: "kubernetes-pod"},
	}
	schemaMap, cueSchemaMap, err := loadSchemas(ctx, refs, schema.DefaultRegistry())
	require.NoError(t, err)
	store := NewResourceStore(map[string]any{}, nil, schemaMap, cueSchemaMap, evaluator.DefaultRegistry())

	handler := handleValidatePolicy(store)

	tests := []struct {
		name          string
		policyFile    string
		platform      string
		wantValid     bool
		wantSyntaxErr bool
		wantContract  bool
	}{
		{
			name:          "valid policy",
			policyFile:    "testdata/policies/valid.rego",
			platform:      "kubernetes-pod",
			wantValid:     true,
			wantSyntaxErr: false,
			wantContract:  false,
		},
		{
			name:          "syntax error",
			policyFile:    "testdata/policies/syntax-error.rego",
			platform:      "kubernetes-pod",
			wantValid:     false,
			wantSyntaxErr: true,
			wantContract:  false,
		},
		{
			name:          "contract violation",
			policyFile:    "testdata/policies/contract-violation.rego",
			platform:      "kubernetes-pod",
			wantValid:     false,
			wantSyntaxErr: false,
			wantContract:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Read policy file
			policyContent, err := os.ReadFile(tt.policyFile)
			require.NoError(t, err)

			// Build request
			input := map[string]interface{}{
				"policyContent": string(policyContent),
				"platform":      tt.platform,
			}
			inputJSON, err := json.Marshal(input)
			require.NoError(t, err)

			req := &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{
					Name:      "validate_policy",
					Arguments: inputJSON,
				},
			}

			// Call handler
			ctx := context.Background()
			result, err := handler(ctx, req)
			require.NoError(t, err)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.wantValid, response["valid"])

			if tt.wantSyntaxErr {
				syntaxErrs := response["syntaxErrors"].([]interface{})
				assert.NotEmpty(t, syntaxErrs)
			}

			if tt.wantContract {
				violations := response["contractViolations"].([]interface{})
				assert.NotEmpty(t, violations)
			}
		})
	}
}

func TestHandleTestPolicy(t *testing.T) {
	// Create resource store with kubernetes schemas
	ctx := context.Background()
	refs := []config.SchemaRef{
		{Platform: "kubernetes-deployment"},
		{Platform: "kubernetes-pod"},
	}
	schemaMap, cueSchemaMap, err := loadSchemas(ctx, refs, schema.DefaultRegistry())
	require.NoError(t, err)
	store := NewResourceStore(map[string]any{}, nil, schemaMap, cueSchemaMap, evaluator.DefaultRegistry())

	handler := handleTestPolicy(store)

	tests := []struct {
		name              string
		policyFile        string
		testData          map[string]interface{}
		platform          string
		wantDataValid     bool
		wantTestsExecuted bool
	}{
		{
			name:       "valid test data",
			policyFile: "testdata/policies/valid.rego",
			testData: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name": "test-pod",
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "test",
							"image": "nginx:latest",
						},
					},
				},
			},
			platform:          "kubernetes-pod",
			wantDataValid:     true,
			wantTestsExecuted: true,
		},
		{
			name:       "invalid platform",
			policyFile: "testdata/policies/valid.rego",
			testData: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
			},
			platform:          "unknown",
			wantDataValid:     false,
			wantTestsExecuted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Read policy file
			policyContent, err := os.ReadFile(tt.policyFile)
			require.NoError(t, err)

			// Build request
			input := map[string]interface{}{
				"policyContent": string(policyContent),
				"testData":      tt.testData,
				"platform":      tt.platform,
			}
			inputJSON, err := json.Marshal(input)
			require.NoError(t, err)

			req := &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{
					Name:      "test_policy",
					Arguments: inputJSON,
				},
			}

			// Call handler
			ctx := context.Background()
			result, err := handler(ctx, req)
			require.NoError(t, err)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.wantDataValid, response["testDataValid"])
			assert.Equal(t, tt.wantTestsExecuted, response["testsExecuted"])
		})
	}
}
