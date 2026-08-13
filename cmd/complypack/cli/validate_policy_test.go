// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePolicyCommand(t *testing.T) {
	root := New()

	t.Run("command exists", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"validate-policy"})
		require.NoError(t, err)
		assert.Equal(t, "validate-policy", cmd.Name())
		assert.NotEmpty(t, cmd.Short)
	})

	t.Run("has flags", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"validate-policy"})
		require.NoError(t, err)

		flags := cmd.Flags()
		assert.NotNil(t, flags.Lookup("platform"), "should have --platform flag")
		assert.NotNil(t, flags.Lookup("evaluator"), "should have --evaluator flag")
		assert.NotNil(t, flags.Lookup("schema"), "should have --schema flag")
		assert.NotNil(t, flags.Lookup("format"), "should have --format flag")
	})

	t.Run("platform is required", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"validate-policy"})
		require.NoError(t, err)

		platformFlag := cmd.Flags().Lookup("platform")
		require.NotNil(t, platformFlag)

		annot, ok := platformFlag.Annotations["cobra_annotation_bash_completion_one_required_flag"]
		assert.True(t, ok, "--platform should be marked required (annotation: %v)", annot)
	})

	t.Run("requires exactly one argument", func(t *testing.T) {
		root := New()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs([]string{"validate-policy", "--platform", "kubernetes-deployment"})

		err := root.Execute()
		assert.Error(t, err, "should error when no positional argument is given")
	})
}

func TestValidatePolicyCommand_MissingPlatform(t *testing.T) {
	dir := t.TempDir()
	policyPath := writePolicyFile(t, dir, regoValidPolicyCLI)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"validate-policy", policyPath})

	err := root.Execute()
	assert.Error(t, err, "should error when --platform is missing")
}

func TestValidatePolicyCommand_FileNotFound(t *testing.T) {
	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"validate-policy", "nonexistent.rego",
		"--platform", "kubernetes-deployment",
		"--format", "json",
	})

	err := root.Execute()
	assert.Error(t, err, "should error when policy file does not exist")
	assert.Contains(t, err.Error(), "reading policy file")
}

func TestValidatePolicyCommand_InvalidEvaluator(t *testing.T) {
	dir := t.TempDir()
	policyPath := writePolicyFile(t, dir, regoValidPolicyCLI)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"validate-policy", policyPath,
		"--platform", "kubernetes-pod",
		"--evaluator", "nonexistent",
		"--format", "json",
	})

	err := root.Execute()
	assert.Error(t, err, "should error when evaluator is not found")
	assert.Contains(t, err.Error(), "nonexistent")
}

// --- Output formatter tests ---

func TestWriteValidatePolicyJSON_Valid(t *testing.T) {
	result := validatePolicyResult{
		valid:              true,
		syntaxErrors:       []string{},
		contractViolations: []map[string]string{},
		lintWarnings:       []map[string]string{},
	}

	var buf bytes.Buffer
	err := writeValidatePolicyJSON(&buf, result)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, true, parsed["valid"])
	assert.Empty(t, parsed["syntaxErrors"])
	assert.Empty(t, parsed["contractViolations"])
	assert.Empty(t, parsed["lintWarnings"])
}

func TestWriteValidatePolicyJSON_SyntaxErrors(t *testing.T) {
	result := validatePolicyResult{
		valid:              false,
		syntaxErrors:       []string{"syntax error at line 5"},
		contractViolations: []map[string]string{},
		lintWarnings:       []map[string]string{},
	}

	var buf bytes.Buffer
	err := writeValidatePolicyJSON(&buf, result)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	assert.Equal(t, false, parsed["valid"])
	syntaxErrs := parsed["syntaxErrors"].([]interface{})
	assert.Len(t, syntaxErrs, 1)
	assert.Contains(t, syntaxErrs[0].(string), "syntax error")
}

func TestWriteValidatePolicyJSON_ContractViolations(t *testing.T) {
	result := validatePolicyResult{
		valid:        false,
		syntaxErrors: []string{},
		contractViolations: []map[string]string{
			{"path": "input.invalid.field", "location": "policy.rego:10:5"},
		},
		lintWarnings: []map[string]string{},
	}

	var buf bytes.Buffer
	err := writeValidatePolicyJSON(&buf, result)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	assert.Equal(t, false, parsed["valid"])
	violations := parsed["contractViolations"].([]interface{})
	assert.Len(t, violations, 1)
	v := violations[0].(map[string]interface{})
	assert.Equal(t, "input.invalid.field", v["path"])
	assert.Equal(t, "policy.rego:10:5", v["location"])
}

func TestWriteValidatePolicyJSON_LintWarnings(t *testing.T) {
	result := validatePolicyResult{
		valid:              true,
		syntaxErrors:       []string{},
		contractViolations: []map[string]string{},
		lintWarnings: []map[string]string{
			{"rule": "no-unused-vars", "message": "unused var x", "location": "policy.rego:3:2"},
		},
	}

	var buf bytes.Buffer
	err := writeValidatePolicyJSON(&buf, result)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	// Lint warnings don't affect validity
	assert.Equal(t, true, parsed["valid"])
	warnings := parsed["lintWarnings"].([]interface{})
	assert.Len(t, warnings, 1)
}

func TestWriteValidatePolicyText_Valid(t *testing.T) {
	result := validatePolicyResult{
		valid:              true,
		syntaxErrors:       []string{},
		contractViolations: []map[string]string{},
		lintWarnings:       []map[string]string{},
	}

	var buf bytes.Buffer
	err := writeValidatePolicyText(&buf, result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "[VALID]")
	assert.Contains(t, output, "Policy is valid")
}

func TestWriteValidatePolicyText_Invalid(t *testing.T) {
	result := validatePolicyResult{
		valid:        false,
		syntaxErrors: []string{"syntax error at line 5"},
		contractViolations: []map[string]string{
			{"path": "input.bad", "location": "p.rego:1:1"},
		},
		lintWarnings: []map[string]string{
			{"rule": "rule1", "message": "msg", "location": "p.rego:2:2"},
		},
	}

	var buf bytes.Buffer
	err := writeValidatePolicyText(&buf, result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "[INVALID]")
	assert.Contains(t, output, "[SYNTAX ERROR]")
	assert.Contains(t, output, "[CONTRACT VIOLATION]")
	assert.Contains(t, output, "[LINT WARNING]")
	// Must NOT contain Unicode symbols
	assert.NotContains(t, output, "✓")
	assert.NotContains(t, output, "✗")
	assert.NotContains(t, output, "⚠")
}

func TestWriteValidatePolicyHuman_Valid(t *testing.T) {
	result := validatePolicyResult{
		valid:              true,
		syntaxErrors:       []string{},
		contractViolations: []map[string]string{},
		lintWarnings:       []map[string]string{},
	}

	var buf bytes.Buffer
	err := writeValidatePolicyHuman(&buf, result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "✓ Policy is valid")
}

func TestWriteValidatePolicyHuman_Invalid(t *testing.T) {
	result := validatePolicyResult{
		valid:        false,
		syntaxErrors: []string{"parse error"},
		contractViolations: []map[string]string{
			{"path": "input.bad", "location": "p.rego:1:1"},
		},
		lintWarnings: []map[string]string{
			{"rule": "r", "message": "m", "location": "p.rego:2:2"},
		},
	}

	var buf bytes.Buffer
	err := writeValidatePolicyHuman(&buf, result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "✗ Policy is invalid")
	assert.Contains(t, output, "Syntax Errors")
	assert.Contains(t, output, "Contract Violations")
	assert.Contains(t, output, "Lint Warnings")
	assert.Contains(t, output, "⚠")
}

// --- End-to-end tests ---

func TestValidatePolicyEndToEnd_ValidPolicy(t *testing.T) {
	dir := t.TempDir()
	policyPath := writePolicyFile(t, dir, regoValidPolicyCLI)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"validate-policy", policyPath,
		"--platform", "kubernetes-pod",
		"--format", "json",
	})

	err := root.Execute()
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(stdout.Bytes(), &parsed)
	require.NoError(t, err, "output should be valid JSON")
	assert.Equal(t, true, parsed["valid"])
	assert.Empty(t, parsed["syntaxErrors"])
	assert.Empty(t, parsed["contractViolations"])
}

func TestValidatePolicyEndToEnd_SyntaxError(t *testing.T) {
	dir := t.TempDir()
	policyPath := writePolicyFile(t, dir, regoSyntaxErrorPolicyCLI)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"validate-policy", policyPath,
		"--platform", "kubernetes-pod",
		"--format", "json",
	})

	err := root.Execute()
	require.ErrorIs(t, err, errPolicyInvalid,
		"should return non-zero exit for invalid policy")

	var parsed map[string]interface{}
	err = json.Unmarshal(stdout.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Equal(t, false, parsed["valid"])
	syntaxErrs := parsed["syntaxErrors"].([]interface{})
	assert.NotEmpty(t, syntaxErrs)
	// Contract violations should be empty (skipped due to syntax errors)
	assert.Empty(t, parsed["contractViolations"])
}

func TestValidatePolicyEndToEnd_ContractViolation(t *testing.T) {
	dir := t.TempDir()
	policyPath := writePolicyFile(t, dir, regoContractViolationPolicyCLI)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"validate-policy", policyPath,
		"--platform", "kubernetes-pod",
		"--format", "json",
	})

	err := root.Execute()
	require.ErrorIs(t, err, errPolicyInvalid,
		"should return non-zero exit for invalid policy")

	var parsed map[string]interface{}
	err = json.Unmarshal(stdout.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Equal(t, false, parsed["valid"])
	violations := parsed["contractViolations"].([]interface{})
	assert.NotEmpty(t, violations)
}

func TestValidatePolicyEndToEnd_TextFormat(t *testing.T) {
	dir := t.TempDir()
	policyPath := writePolicyFile(t, dir, regoValidPolicyCLI)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"validate-policy", policyPath,
		"--platform", "kubernetes-pod",
		"--format", "text",
	})

	err := root.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "[VALID]")
}

func TestValidatePolicyEndToEnd_HumanFormat(t *testing.T) {
	dir := t.TempDir()
	policyPath := writePolicyFile(t, dir, regoValidPolicyCLI)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"validate-policy", policyPath,
		"--platform", "kubernetes-pod",
		"--format", "human",
	})

	err := root.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "✓ Policy is valid")
}

func TestValidatePolicyEndToEnd_InvalidFormat(
	t *testing.T,
) {
	dir := t.TempDir()
	policyPath := writePolicyFile(
		t, dir, regoValidPolicyCLI,
	)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"validate-policy", policyPath,
		"--platform", "kubernetes-pod",
		"--format", "invalid-fmt",
	})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid-fmt")
}
