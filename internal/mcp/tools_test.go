// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

// Inline Rego policy strings for wiring tests.
// These replace the former testdata/policies/ files to avoid duplication
// with internal/prepack/testdata/ (see design in issue #151).

const regoValidPolicy = `package mcp.valid

import rego.v1

deny contains msg if {
	input.kind == "Pod"
	not input.metadata.name
	msg := "Pods must have a name"
}`

const regoSyntaxErrorPolicy = `package mcp.syntax_error

import rego.v1

deny contains msg if {
	input.kind == "Pod"
	not input.metadata.labels["app"]
	msg := "Pods must have 'app' label"
  # Missing closing brace`

const regoContractViolationPolicy = `package mcp.contract_violation

import rego.v1

deny contains msg if {
	input.kind == "Pod"
	# This field doesn't exist in Kubernetes schema
	not input.metadata.invalid_field
	msg := "Contract violation example"
}`

// testLoadAllSchemas loads a representative subset of schemas for testing.
func testLoadAllSchemas(t *testing.T) (map[string][]byte, map[string]cue.Value) {
	t.Helper()
	ctx := context.Background()

	refs := []config.SchemaRef{
		{Platform: "kubernetes-deployment"},
	}

	schemaMap, cueSchemaMap, err := schema.LoadFromConfig(ctx, refs, schema.DefaultRegistry())
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

func TestValidateDataWithStore(t *testing.T) {
	// Create resource store with schemas
	schemaMap, cueSchemaMap := testLoadAllSchemas(t)
	store := NewResourceStore(
		map[string]any{}, nil, schemaMap,
		cueSchemaMap, evaluator.DefaultRegistry(),
	)

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
			errContains: "no CUE schema loaded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cueSchema, err := store.CUESchema(
				tt.platform,
			)
			if err != nil {
				if tt.wantErrors {
					assert.Contains(t,
						err.Error(),
						tt.errContains,
					)
					return
				}
				require.NoError(t, err)
			}

			errs := schema.ValidateData(
				tt.testData, cueSchema,
			)
			if tt.wantErrors {
				assert.NotEmpty(t, errs,
					"expected validation errors")
			} else {
				assert.Empty(t, errs,
					"expected no validation errors")
			}
		})
	}
}

func TestBuildValidationResponse(t *testing.T) {
	tests := []struct {
		name      string
		input     *evaluator.ValidatePolicyResult
		wantValid bool
	}{
		{
			name: "valid policy",
			input: &evaluator.ValidatePolicyResult{
				Valid:              true,
				SyntaxErrors:       []string{},
				ContractViolations: nil,
				LintWarnings:       nil,
			},
			wantValid: true,
		},
		{
			name: "syntax errors",
			input: &evaluator.ValidatePolicyResult{
				Valid: false,
				SyntaxErrors: []string{
					"syntax error at line 5",
				},
				ContractViolations: nil,
				LintWarnings:       nil,
			},
			wantValid: false,
		},
		{
			name: "contract violations",
			input: &evaluator.ValidatePolicyResult{
				Valid:        false,
				SyntaxErrors: []string{},
				ContractViolations: []evaluator.ContractViolation{
					{
						Path:     "input.invalid.field",
						Location: "policy.rego:10:5",
					},
				},
				LintWarnings: nil,
			},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildValidationResponse(
				tt.input,
			)
			require.NoError(t, err)

			textContent, ok := result.Content[0].(*mcp.TextContent)
			require.True(t, ok, "expected TextContent")

			var response map[string]interface{}
			err = json.Unmarshal(
				[]byte(textContent.Text), &response,
			)
			require.NoError(t, err)

			assert.Equal(t,
				tt.wantValid, response["valid"],
			)
		})
	}
}

func TestBuildTestPolicyResponse_DataError(t *testing.T) {
	input := &evaluator.TestPolicyResult{
		TestDataValid: false,
		TestDataErrors: []string{
			"input.kind: invalid value",
			"input.metadata.name: required",
		},
		TestsExecuted: false,
	}

	result, err := buildTestPolicyResponse(input)
	require.NoError(t, err)

	var response map[string]interface{}
	err = json.Unmarshal(
		[]byte(
			result.Content[0].(*mcp.TextContent).Text,
		),
		&response,
	)
	require.NoError(t, err)

	assert.False(t, response["testDataValid"].(bool))
	assert.False(t, response["testsExecuted"].(bool))
	testDataErrs := response["testDataErrors"].([]interface{})
	assert.Len(t, testDataErrs, 2)
}

func TestBuildTestPolicyResponse_Results(t *testing.T) {
	input := &evaluator.TestPolicyResult{
		TestDataValid:  true,
		TestDataErrors: []string{},
		TestsExecuted:  true,
		Results: &evaluator.TestResults{
			Total:  5,
			Passed: 3,
			Failed: 2,
			Errors: []string{
				"test_deny_root: expected denial",
				"test_labels: assertion failed",
			},
		},
	}

	result, err := buildTestPolicyResponse(input)
	require.NoError(t, err)

	var response map[string]interface{}
	err = json.Unmarshal(
		[]byte(
			result.Content[0].(*mcp.TextContent).Text,
		),
		&response,
	)
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
	schemaMap, cueSchemaMap, err := schema.LoadFromConfig(ctx, refs, schema.DefaultRegistry())
	require.NoError(t, err)
	store := NewResourceStore(map[string]any{}, nil, schemaMap, cueSchemaMap, evaluator.DefaultRegistry())

	handler := handleValidatePolicy(store)

	tests := []struct {
		name          string
		policyContent string
		platform      string
		wantValid     bool
		wantSyntaxErr bool
		wantContract  bool
	}{
		{
			name:          "valid policy",
			policyContent: regoValidPolicy,
			platform:      "kubernetes-pod",
			wantValid:     true,
			wantSyntaxErr: false,
			wantContract:  false,
		},
		{
			name:          "syntax error",
			policyContent: regoSyntaxErrorPolicy,
			platform:      "kubernetes-pod",
			wantValid:     false,
			wantSyntaxErr: true,
			wantContract:  false,
		},
		{
			name:          "contract violation",
			policyContent: regoContractViolationPolicy,
			platform:      "kubernetes-pod",
			wantValid:     false,
			wantSyntaxErr: false,
			wantContract:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build request
			input := map[string]interface{}{
				"policyContent": tt.policyContent,
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

func TestCreateValidateConfigTool(t *testing.T) {
	tool := createValidateConfigTool()

	assert.Equal(t, "validate_config", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, tool.InputSchema)

	schema := tool.InputSchema.(map[string]interface{})
	props := schema["properties"].(map[string]interface{})

	assert.Contains(t, props, "path")
	assert.Contains(t, props, "unknownFields")
	assert.Contains(t, props, "scope")

	required := schema["required"].([]interface{})
	assert.Contains(t, required, "path")
}

func TestHandleValidateConfig(t *testing.T) {
	handler := handleValidateConfig()

	t.Run("valid config returns structured JSON", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "complypack.yaml")
		require.NoError(t, os.WriteFile(cfgPath, []byte("version: 0.1.0\nschemas:\n  - platform: kubernetes-deployment\n"), 0600))

		input := map[string]interface{}{
			"path": cfgPath,
		}
		inputJSON, err := json.Marshal(input)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name:      "validate_config",
				Arguments: inputJSON,
			},
		}

		result, err := handler(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, result.IsError)

		var response map[string]interface{}
		err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &response)
		require.NoError(t, err)

		// Minimal config: schema validation passes but scope checks may fail
		assert.False(t, response["valid"].(bool), "minimal config fails scope validation")
		errors := response["errors"].([]interface{})
		assert.Empty(t, errors, "no schema-level errors")

		scopes := response["scopes"].([]interface{})
		assert.Len(t, scopes, 3, "default validates all 3 scopes")
	})

	t.Run("invalid config returns structured error", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "complypack.yaml")
		require.NoError(t, os.WriteFile(cfgPath, []byte("version: nope\n"), 0600))

		input := map[string]interface{}{
			"path": cfgPath,
		}
		inputJSON, err := json.Marshal(input)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name:      "validate_config",
				Arguments: inputJSON,
			},
		}

		result, err := handler(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, result.IsError, "validation failures use structured JSON, not IsError")

		var response map[string]interface{}
		err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &response)
		require.NoError(t, err)

		assert.False(t, response["valid"].(bool))
		errors := response["errors"].([]interface{})
		assert.NotEmpty(t, errors)
		assert.Contains(t, errors[0].(string), "schema validation", "error should mention schema validation")
	})

	t.Run("unknownFields error mode", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "complypack.yaml")
		require.NoError(t, os.WriteFile(cfgPath, []byte("version: 0.1.0\nschemas:\n  - platform: kubernetes-deployment\nbogus: true\n"), 0600))

		input := map[string]interface{}{
			"path":          cfgPath,
			"unknownFields": "error",
		}
		inputJSON, err := json.Marshal(input)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name:      "validate_config",
				Arguments: inputJSON,
			},
		}

		result, err := handler(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, result.IsError, "validation failures use structured JSON, not IsError")

		var response map[string]interface{}
		err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &response)
		require.NoError(t, err)

		assert.False(t, response["valid"].(bool))
		errors := response["errors"].([]interface{})
		assert.NotEmpty(t, errors, "strict mode should produce errors for unknown fields")
	})

	t.Run("scope filtering", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "complypack.yaml")
		require.NoError(t, os.WriteFile(cfgPath, []byte("version: 0.1.0\nschemas:\n  - platform: kubernetes-deployment\n"), 0600))

		input := map[string]interface{}{
			"path":  cfgPath,
			"scope": []string{"pack"},
		}
		inputJSON, err := json.Marshal(input)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name:      "validate_config",
				Arguments: inputJSON,
			},
		}

		result, err := handler(context.Background(), req)
		require.NoError(t, err)

		var response map[string]interface{}
		err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &response)
		require.NoError(t, err)

		scopes := response["scopes"].([]interface{})
		assert.Len(t, scopes, 1, "should only validate requested scope")

		scopeMap := scopes[0].(map[string]interface{})
		assert.Equal(t, "pack", scopeMap["scope"])
		assert.False(t, scopeMap["valid"].(bool), "pack scope should fail: missing id")
	})
}

func TestHandleTestPolicy(t *testing.T) {
	// Create resource store with kubernetes schemas
	ctx := context.Background()
	refs := []config.SchemaRef{
		{Platform: "kubernetes-deployment"},
		{Platform: "kubernetes-pod"},
	}
	schemaMap, cueSchemaMap, err := schema.LoadFromConfig(ctx, refs, schema.DefaultRegistry())
	require.NoError(t, err)
	store := NewResourceStore(map[string]any{}, nil, schemaMap, cueSchemaMap, evaluator.DefaultRegistry())

	handler := handleTestPolicy(store)

	tests := []struct {
		name              string
		policyContent     string
		testData          map[string]interface{}
		platform          string
		wantDataValid     bool
		wantTestsExecuted bool
	}{
		{
			name:          "valid test data",
			policyContent: regoValidPolicy,
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
			name:          "invalid platform",
			policyContent: regoValidPolicy,
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
			// Build request
			input := map[string]interface{}{
				"policyContent": tt.policyContent,
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
