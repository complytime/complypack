// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/complytime/complypack/internal/coverage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverageCommand(t *testing.T) {
	root := New()

	coverageCmd, _, err := root.Find([]string{"coverage"})
	require.NoError(t, err, "coverage command should exist")
	assert.Equal(t, "coverage", coverageCmd.Name())
	assert.NotEmpty(t, coverageCmd.Short, "coverage command should have a short description")

	flags := coverageCmd.Flags()
	assert.NotNil(t, flags.Lookup("policy"), "should have --policy flag")
	assert.NotNil(t, flags.Lookup("policy-dir"), "should have --policy-dir flag")
	assert.NotNil(t, flags.Lookup("config"), "should have --config flag")
	assert.NotNil(t, flags.Lookup("evaluator"), "should have --evaluator flag")
	assert.NotNil(t, flags.Lookup("run-tests"), "should have --run-tests flag")
	assert.NotNil(t, flags.Lookup("format"), "should have --format flag")
	assert.NotNil(t, flags.Lookup("source"), "should have --source flag")
	assert.NotNil(t, flags.Lookup("cache-dir"), "should have --cache-dir flag")
}

func TestCoverageCommand_MissingRequiredFlags(t *testing.T) {
	root := New()

	root.SetArgs([]string{"coverage"})

	err := root.Execute()
	assert.Error(t, err, "should error when required flags are missing")
}

func TestWriteHuman(t *testing.T) {
	report := &coverage.Report{
		PolicyID: "test-policy",
		Requirements: []coverage.RequirementEntry{
			{RequirementID: "CTL-001-AR1", ControlID: "CTL-001", Status: coverage.StatusImplemented},
			{RequirementID: "CTL-001-AR2", ControlID: "CTL-001", Status: coverage.StatusGap},
			{RequirementID: "CTL-002-AR1", ControlID: "CTL-002", Status: coverage.StatusImplementedPassing},
		},
		Metrics: coverage.Metrics{
			TotalAutomated:  3,
			Implemented:     2,
			Gaps:            1,
			CoveragePercent: 66.7,
			Passing:         1,
		},
		Warnings: []coverage.Warning{},
	}

	var buf bytes.Buffer
	err := writeHuman(&buf, report)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "test-policy")
	assert.Contains(t, output, "CTL-001")
	assert.Contains(t, output, "CTL-002")
	assert.Contains(t, output, "CTL-001-AR1")
	assert.Contains(t, output, "● OK")
	assert.Contains(t, output, "○ GAP")
	assert.Contains(t, output, "✓ PASS")
	assert.Contains(t, output, "2/3 requirements covered")
	assert.Contains(t, output, "Gaps: 1")
}

func TestWriteHuman_WithTestResults(t *testing.T) {
	report := &coverage.Report{
		PolicyID: "test-policy",
		Requirements: []coverage.RequirementEntry{
			{RequirementID: "R1", ControlID: "C1", Status: coverage.StatusImplementedPassing},
			{RequirementID: "R2", ControlID: "C1", Status: coverage.StatusImplementedFailing},
		},
		Metrics: coverage.Metrics{
			TotalAutomated:  2,
			Implemented:     2,
			CoveragePercent: 100,
			Passing:         1,
			Failing:         1,
		},
	}

	var buf bytes.Buffer
	err := writeHuman(&buf, report)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "✓ PASS")
	assert.Contains(t, output, "✗ FAIL")
	assert.Contains(t, output, "Passing")
	assert.Contains(t, output, "Failing")
}

func TestWritePlainText(t *testing.T) {
	report := &coverage.Report{
		PolicyID: "test-policy",
		Requirements: []coverage.RequirementEntry{
			{RequirementID: "CTL-001-AR1", ControlID: "CTL-001", Status: coverage.StatusImplemented},
			{RequirementID: "CTL-001-AR2", ControlID: "CTL-001", Status: coverage.StatusGap},
			{RequirementID: "CTL-002-AR1", ControlID: "CTL-002", Status: coverage.StatusImplementedPassing},
		},
		Metrics: coverage.Metrics{
			TotalAutomated:  3,
			Implemented:     2,
			Gaps:            1,
			CoveragePercent: 66.7,
			Passing:         1,
		},
		Warnings: []coverage.Warning{},
	}

	var buf bytes.Buffer
	err := writePlainText(&buf, report)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Coverage Report: test-policy")
	assert.Contains(t, output, "CTL-001")
	assert.Contains(t, output, "CTL-002")
	assert.Contains(t, output, "[OK]")
	assert.Contains(t, output, "[GAP]")
	assert.Contains(t, output, "[PASS]")
	assert.Contains(t, output, "2/3 requirements covered")
	assert.Contains(t, output, "Gaps: 1")
	// Must NOT contain Unicode symbols
	assert.NotContains(t, output, "✓")
	assert.NotContains(t, output, "✗")
	assert.NotContains(t, output, "●")
	assert.NotContains(t, output, "○")
	assert.NotContains(t, output, "━")
	assert.NotContains(t, output, "⚠")
}

func TestWritePlainText_WithWarnings(t *testing.T) {
	report := &coverage.Report{
		PolicyID:     "test-policy",
		Requirements: []coverage.RequirementEntry{},
		Metrics:      coverage.Metrics{},
		Warnings: []coverage.Warning{
			{Message: "something went wrong"},
		},
	}

	var buf bytes.Buffer
	err := writePlainText(&buf, report)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "WARNING: something went wrong")
}

func TestWritePlainText_WithTestResults(t *testing.T) {
	report := &coverage.Report{
		PolicyID: "test-policy",
		Requirements: []coverage.RequirementEntry{
			{RequirementID: "R1", ControlID: "C1", Status: coverage.StatusImplementedPassing},
			{RequirementID: "R2", ControlID: "C1", Status: coverage.StatusImplementedFailing},
		},
		Metrics: coverage.Metrics{
			TotalAutomated:  2,
			Implemented:     2,
			CoveragePercent: 100,
			Passing:         1,
			Failing:         1,
		},
	}

	var buf bytes.Buffer
	err := writePlainText(&buf, report)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "[PASS]")
	assert.Contains(t, output, "[FAIL]")
	assert.Contains(t, output, "Passing: 1  Failing: 1")
}

func TestWriteJSON(t *testing.T) {
	report := &coverage.Report{
		PolicyID: "test-policy",
		Requirements: []coverage.RequirementEntry{
			{RequirementID: "CTL-001-AR1", ControlID: "CTL-001", Status: coverage.StatusImplemented},
			{RequirementID: "CTL-002-AR1", ControlID: "CTL-002", Status: coverage.StatusGap},
		},
		Metrics: coverage.Metrics{
			TotalAutomated:  2,
			Implemented:     1,
			Gaps:            1,
			CoveragePercent: 50,
		},
		Warnings: []coverage.Warning{},
	}

	var buf bytes.Buffer
	err := writeJSON(&buf, report)
	require.NoError(t, err)

	var parsed coverage.Report
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, "test-policy", parsed.PolicyID)
	assert.Equal(t, 2, len(parsed.Requirements))
	assert.Equal(t, 50.0, parsed.Metrics.CoveragePercent)
	assert.Equal(t, 1, parsed.Metrics.Gaps)
}

func TestStatusIndicator(t *testing.T) {
	tests := []struct {
		status   coverage.RequirementStatus
		contains string
	}{
		{coverage.StatusImplementedPassing, "✓ PASS"},
		{coverage.StatusImplementedFailing, "✗ FAIL"},
		{coverage.StatusImplemented, "● OK"},
		{coverage.StatusGap, "○ GAP"},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			got := statusIndicator(tc.status)
			assert.Contains(t, got, tc.contains)
		})
	}
}

func TestPlainStatusIndicator(t *testing.T) {
	tests := []struct {
		status   coverage.RequirementStatus
		expected string
	}{
		{coverage.StatusImplementedPassing, "[PASS]"},
		{coverage.StatusImplementedFailing, "[FAIL]"},
		{coverage.StatusImplemented, "[OK]  "},
		{coverage.StatusGap, "[GAP] "},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			got := plainStatusIndicator(tc.status)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestResolveFormat(t *testing.T) {
	t.Run("explicit human", func(t *testing.T) {
		f, err := resolveFormat("human")
		require.NoError(t, err)
		assert.Equal(t, formatHuman, f)
	})

	t.Run("explicit text", func(t *testing.T) {
		f, err := resolveFormat("text")
		require.NoError(t, err)
		assert.Equal(t, formatText, f)
	})

	t.Run("explicit json", func(t *testing.T) {
		f, err := resolveFormat("json")
		require.NoError(t, err)
		assert.Equal(t, formatJSON, f)
	})

	t.Run("invalid format", func(t *testing.T) {
		_, err := resolveFormat("yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown format")
	})

	t.Run("empty defaults to human when NO_COLOR unset", func(t *testing.T) {
		// Ensure NO_COLOR is not set for this subtest
		t.Setenv("NO_COLOR", "")
		f, err := resolveFormat("")
		require.NoError(t, err)
		// NO_COLOR="" (empty string) means os.Getenv returns "" which
		// does not trigger text mode — matches complyctl behavior
		assert.Equal(t, formatHuman, f)
	})

	t.Run("NO_COLOR defaults to text", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		f, err := resolveFormat("")
		require.NoError(t, err)
		assert.Equal(t, formatText, f)
	})
}
