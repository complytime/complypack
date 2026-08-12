// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/complytime/complypack/internal/evaluator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestPolicyCommand(t *testing.T) {
	root := New()

	t.Run("command exists", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"test-policy"})
		require.NoError(t, err)
		assert.Equal(t, "test-policy", cmd.Name())
		assert.NotEmpty(t, cmd.Short)
	})

	t.Run("has flags", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"test-policy"})
		require.NoError(t, err)

		flags := cmd.Flags()
		assert.NotNil(t, flags.Lookup("platform"), "should have --platform flag")
		assert.NotNil(t, flags.Lookup("evaluator"), "should have --evaluator flag")
		assert.NotNil(t, flags.Lookup("schema"), "should have --schema flag")
		assert.NotNil(t, flags.Lookup("test-data"), "should have --test-data flag")
		assert.NotNil(t, flags.Lookup("format"), "should have --format flag")
	})

	t.Run("platform is required", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"test-policy"})
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
		root.SetArgs([]string{"test-policy", "--platform", "kubernetes-deployment"})

		err := root.Execute()
		assert.Error(t, err, "should error when no positional argument is given")
	})
}

func TestTestPolicyCommand_MissingPlatform(t *testing.T) {
	dir := t.TempDir()
	policyPath := writePolicyFile(t, dir, regoValidPolicyCLI)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"test-policy", policyPath})

	err := root.Execute()
	assert.Error(t, err, "should error when --platform is missing")
}

func TestTestPolicyCommand_FileNotFound(t *testing.T) {
	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"test-policy", "nonexistent.rego",
		"--platform", "kubernetes-deployment",
		"--format", "json",
	})

	err := root.Execute()
	assert.Error(t, err, "should error when policy file does not exist")
	assert.Contains(t, err.Error(), "reading policy file")
}

func TestTestPolicyCommand_TestDataFileNotFound(t *testing.T) {
	dir := t.TempDir()
	policyPath := writePolicyFile(t, dir, regoValidPolicyCLI)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"test-policy", policyPath,
		"--platform", "kubernetes-deployment",
		"--test-data", "nonexistent.json",
		"--format", "json",
	})

	err := root.Execute()
	assert.Error(t, err, "should error when test-data file does not exist")
	assert.Contains(t, err.Error(), "reading test-data file")
}

func TestTestPolicyCommand_InvalidTestDataJSON(t *testing.T) {
	dir := t.TempDir()
	policyPath := writePolicyFile(t, dir, regoValidPolicyCLI)
	testDataPath := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(testDataPath, []byte("not json"), 0600))

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"test-policy", policyPath,
		"--platform", "kubernetes-deployment",
		"--test-data", testDataPath,
		"--format", "json",
	})

	err := root.Execute()
	assert.Error(t, err, "should error on invalid JSON")
	assert.Contains(t, err.Error(), "parsing test-data JSON")
}

// --- Output formatter tests ---

func TestWriteTestPolicyJSON_TestsExecuted(t *testing.T) {
	result := evaluator.TestPolicyResult{
		TestDataValid:  true,
		TestDataErrors: []string{},
		TestsExecuted:  true,
		Results: &evaluator.TestResults{
			Total:  5,
			Passed: 3,
			Failed: 2,
			Errors: []string{"test_deny: expected denial"},
		},
	}

	var buf bytes.Buffer
	err := writeTestPolicyJSON(&buf, &result)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, true, parsed["testDataValid"])
	assert.Equal(t, true, parsed["testsExecuted"])

	results := parsed["results"].(map[string]interface{})
	assert.Equal(t, float64(5), results["total"])
	assert.Equal(t, float64(3), results["passed"])
	assert.Equal(t, float64(2), results["failed"])
	errors := results["errors"].([]interface{})
	assert.Len(t, errors, 1)
}

func TestWriteTestPolicyJSON_TestDataInvalid(t *testing.T) {
	result := evaluator.TestPolicyResult{
		TestDataValid:  false,
		TestDataErrors: []string{"input.kind: invalid value"},
		TestsExecuted:  false,
	}

	var buf bytes.Buffer
	err := writeTestPolicyJSON(&buf, &result)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	assert.Equal(t, false, parsed["testDataValid"])
	assert.Equal(t, false, parsed["testsExecuted"])
	testDataErrs := parsed["testDataErrors"].([]interface{})
	assert.Len(t, testDataErrs, 1)
	assert.Nil(t, parsed["results"], "results should be absent when tests not executed")
}

func TestWriteTestPolicyJSON_NoTestData(t *testing.T) {
	result := evaluator.TestPolicyResult{
		TestDataValid:  true,
		TestDataErrors: []string{},
		TestsExecuted:  true,
		Results: &evaluator.TestResults{
			Total:  1,
			Passed: 1,
			Failed: 0,
			Errors: []string{},
		},
	}

	var buf bytes.Buffer
	err := writeTestPolicyJSON(&buf, &result)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	assert.Equal(t, true, parsed["testDataValid"])
	assert.Equal(t, true, parsed["testsExecuted"])
}

func TestWriteTestPolicyText_Pass(t *testing.T) {
	result := evaluator.TestPolicyResult{
		TestDataValid:  true,
		TestDataErrors: []string{},
		TestsExecuted:  true,
		Results: &evaluator.TestResults{
			Total:  3,
			Passed: 3,
			Failed: 0,
			Errors: []string{},
		},
	}

	var buf bytes.Buffer
	err := writeTestPolicyText(&buf, &result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "[PASS]")
	assert.Contains(t, output, "3/3 tests passed")
}

func TestWriteTestPolicyText_Fail(t *testing.T) {
	result := evaluator.TestPolicyResult{
		TestDataValid:  true,
		TestDataErrors: []string{},
		TestsExecuted:  true,
		Results: &evaluator.TestResults{
			Total:  3,
			Passed: 1,
			Failed: 2,
			Errors: []string{"test_a: failed", "test_b: failed"},
		},
	}

	var buf bytes.Buffer
	err := writeTestPolicyText(&buf, &result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "[FAIL]")
	assert.Contains(t, output, "1/3 tests passed")
	assert.Contains(t, output, "[ERROR]")
}

func TestWriteTestPolicyText_DataInvalid(t *testing.T) {
	result := evaluator.TestPolicyResult{
		TestDataValid:  false,
		TestDataErrors: []string{"bad field"},
		TestsExecuted:  false,
	}

	var buf bytes.Buffer
	err := writeTestPolicyText(&buf, &result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "[FAIL] Test data validation failed")
	assert.Contains(t, output, "[TEST DATA ERROR]")
}

func TestWriteTestPolicyText_NotExecuted(t *testing.T) {
	result := evaluator.TestPolicyResult{
		TestDataValid:  true,
		TestDataErrors: []string{},
		TestsExecuted:  false,
	}

	var buf bytes.Buffer
	err := writeTestPolicyText(&buf, &result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "[SKIP] Tests were not executed")
}

func TestWriteTestPolicyHuman_Pass(t *testing.T) {
	result := evaluator.TestPolicyResult{
		TestDataValid:  true,
		TestDataErrors: []string{},
		TestsExecuted:  true,
		Results: &evaluator.TestResults{
			Total:  2,
			Passed: 2,
			Failed: 0,
			Errors: []string{},
		},
	}

	var buf bytes.Buffer
	err := writeTestPolicyHuman(&buf, &result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "✓ 2/2 tests passed")
}

func TestWriteTestPolicyHuman_Fail(t *testing.T) {
	result := evaluator.TestPolicyResult{
		TestDataValid:  true,
		TestDataErrors: []string{},
		TestsExecuted:  true,
		Results: &evaluator.TestResults{
			Total:  3,
			Passed: 1,
			Failed: 2,
			Errors: []string{"test_a: failed"},
		},
	}

	var buf bytes.Buffer
	err := writeTestPolicyHuman(&buf, &result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "✗ 1/3 tests passed")
	assert.Contains(t, output, "Test Errors")
}

func TestWriteTestPolicyHuman_DataInvalid(t *testing.T) {
	result := evaluator.TestPolicyResult{
		TestDataValid:  false,
		TestDataErrors: []string{"error1"},
		TestsExecuted:  false,
	}

	var buf bytes.Buffer
	err := writeTestPolicyHuman(&buf, &result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "✗ Test data validation failed")
	assert.Contains(t, output, "Test Data Errors")
}

func TestWriteTestPolicyHuman_NotExecuted(t *testing.T) {
	result := evaluator.TestPolicyResult{
		TestDataValid:  true,
		TestDataErrors: []string{},
		TestsExecuted:  false,
	}

	var buf bytes.Buffer
	err := writeTestPolicyHuman(&buf, &result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Tests were not executed")
}

// --- End-to-end tests ---

func TestTestPolicyEndToEnd_RunsTests(t *testing.T) {
	dir := t.TempDir()
	policyPath := writePolicyFile(t, dir, regoValidPolicyCLI)

	// Write valid test-data JSON matching kubernetes-pod schema
	testDataPath := filepath.Join(dir, "test-data.json")
	testDataJSON := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {"name": "test-pod"},
		"spec": {
			"containers": [{"name": "c", "image": "nginx:latest"}]
		}
	}`
	require.NoError(t,
		os.WriteFile(testDataPath, []byte(testDataJSON), 0600))

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"test-policy", policyPath,
		"--platform", "kubernetes-pod",
		"--test-data", testDataPath,
		"--format", "json",
	})

	err := root.Execute()
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(stdout.Bytes(), &parsed)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, true, parsed["testDataValid"])
	assert.Equal(t, true, parsed["testsExecuted"])
	assert.NotNil(t, parsed["results"],
		"results should be present when tests execute")
}

func TestTestPolicyEndToEnd_NoTestData(t *testing.T) {
	dir := t.TempDir()
	policyPath := writePolicyFile(t, dir, regoValidPolicyCLI)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"test-policy", policyPath,
		"--platform", "kubernetes-pod",
		"--format", "json",
	})

	err := root.Execute()
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(stdout.Bytes(), &parsed)
	require.NoError(t, err)

	// When --test-data is omitted: testDataValid=true, tests still run
	assert.Equal(t, true, parsed["testDataValid"])
	assert.Equal(t, true, parsed["testsExecuted"])
}

func TestTestPolicyEndToEnd_TextFormat(t *testing.T) {
	dir := t.TempDir()
	policyPath := writePolicyFile(t, dir, regoValidPolicyCLI)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"test-policy", policyPath,
		"--platform", "kubernetes-pod",
		"--format", "text",
	})

	err := root.Execute()
	require.NoError(t, err)

	output := stdout.String()
	// The policy has no test_ rules, so OPA will report 0 tests
	assert.Contains(t, output, "0/0 tests passed")
}

func TestTestPolicyEndToEnd_HumanFormat(t *testing.T) {
	dir := t.TempDir()
	policyPath := writePolicyFile(t, dir, regoValidPolicyCLI)

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"test-policy", policyPath,
		"--platform", "kubernetes-pod",
		"--format", "human",
	})

	err := root.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "0/0 tests passed")
}

func TestTestPolicyEndToEnd_InvalidFormat(
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
		"test-policy", policyPath,
		"--platform", "kubernetes-pod",
		"--format", "bad-format",
	})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad-format")
}

func TestTestPolicyEndToEnd_InvalidTestData(
	t *testing.T,
) {
	dir := t.TempDir()
	policyPath := writePolicyFile(
		t, dir, regoValidPolicyCLI,
	)

	// Write test-data that violates the CUE schema
	testDataPath := filepath.Join(
		dir, "bad-data.json",
	)
	badJSON := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {"name": "test"},
		"spec": {"containers": "not-a-list"}
	}`
	require.NoError(t, os.WriteFile(
		testDataPath, []byte(badJSON), 0600,
	))

	root := New()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"test-policy", policyPath,
		"--platform", "kubernetes-pod",
		"--test-data", testDataPath,
		"--format", "json",
	})

	err := root.Execute()
	require.ErrorIs(t, err, errTestsFailed,
		"should return non-zero exit for bad test data")

	var parsed map[string]interface{}
	err = json.Unmarshal(stdout.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Equal(t, false, parsed["testDataValid"])
	assert.Equal(t, false, parsed["testsExecuted"])

	tdErrors, ok :=
		parsed["testDataErrors"].([]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, tdErrors,
		"should report test data errors")
}
