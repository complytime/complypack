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
	assert.NotNil(t, flags.Lookup("output"), "should have --output flag")
	assert.NotNil(t, flags.Lookup("source"), "should have --source flag")
	assert.NotNil(t, flags.Lookup("cache-dir"), "should have --cache-dir flag")
}

func TestCoverageCommand_MissingRequiredFlags(t *testing.T) {
	root := New()

	root.SetArgs([]string{"coverage"})

	err := root.Execute()
	assert.Error(t, err, "should error when required flags are missing")
}

func TestWriteText(t *testing.T) {
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
	err := writeText(&buf, report)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "test-policy")
	assert.Contains(t, output, "CTL-001")
	assert.Contains(t, output, "CTL-002")
	assert.Contains(t, output, "CTL-001-AR1")
	assert.Contains(t, output, "[ OK ]")
	assert.Contains(t, output, "[ -- ]")
	assert.Contains(t, output, "[PASS]")
	assert.Contains(t, output, "2/3 requirements covered")
	assert.Contains(t, output, "Gaps: 1")
}

func TestWriteText_WithTestResults(t *testing.T) {
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
	err := writeText(&buf, report)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "[PASS]")
	assert.Contains(t, output, "[FAIL]")
	assert.Contains(t, output, "Passing: 1")
	assert.Contains(t, output, "Failing: 1")
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
		status coverage.RequirementStatus
		want   string
	}{
		{coverage.StatusImplementedPassing, "[PASS]"},
		{coverage.StatusImplementedFailing, "[FAIL]"},
		{coverage.StatusImplemented, "[ OK ]"},
		{coverage.StatusGap, "[ -- ]"},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			got := statusIndicator(tc.status)
			assert.Equal(t, tc.want, got)
		})
	}
}
