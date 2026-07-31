// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/complytime/complypack/internal/requirement"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTriageCommand(t *testing.T) {
	root := New()

	cmd, _, err := root.Find([]string{"triage"})
	require.NoError(t, err, "triage command should exist")
	assert.Equal(t, "triage", cmd.Name())
	assert.NotEmpty(t, cmd.Short)

	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("policy"),
		"should have --policy flag")
	assert.NotNil(t, flags.Lookup("config"),
		"should have --config flag")
	assert.NotNil(t, flags.Lookup("cache-dir"),
		"should have --cache-dir flag")
	assert.NotNil(t, flags.Lookup("format"),
		"should have --format flag")
	assert.NotNil(t, flags.Lookup("source"),
		"should have --source flag")

	// triage must NOT have --catalog
	assert.Nil(t, flags.Lookup("catalog"),
		"triage should not have --catalog flag")
}

func TestTriageCommand_MissingRequiredFlags(t *testing.T) {
	root := New()
	root.SetArgs([]string{"triage"})

	err := root.Execute()
	assert.Error(t, err,
		"should error when --policy is missing")
}

func TestWriteTriageText(t *testing.T) {
	result := &requirement.TriageResult{
		PolicyID: "test-policy",
		Automated: []requirement.TriagedPlan{
			{
				PlanID:           "plan-1",
				RequirementID:    "REQ-001",
				EvaluationMethod: "conftest",
				Executor:         "opa",
			},
		},
		Manual: []requirement.TriagedPlan{
			{
				PlanID:        "plan-2",
				RequirementID: "REQ-002",
			},
		},
		Counts: requirement.TriageCounts{
			Automated: 1,
			Manual:    1,
			Total:     2,
		},
	}

	var buf bytes.Buffer
	err := writeTriageText(&buf, result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Triage: test-policy")
	assert.Contains(t, output, "Automated: 1")
	assert.Contains(t, output, "Manual: 1")
	assert.Contains(t, output, "Total: 2")
	assert.Contains(t, output, "plan-1")
	assert.Contains(t, output, "REQ-001")
	assert.Contains(t, output, "conftest")
	assert.Contains(t, output, "plan-2")
	assert.Contains(t, output, "REQ-002")
}

func TestWriteTriageText_NoPlans(t *testing.T) {
	result := &requirement.TriageResult{
		PolicyID:  "empty-policy",
		Automated: []requirement.TriagedPlan{},
		Manual:    []requirement.TriagedPlan{},
		Counts: requirement.TriageCounts{
			Automated: 0,
			Manual:    0,
			Total:     0,
		},
	}

	var buf bytes.Buffer
	err := writeTriageText(&buf, result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Triage: empty-policy")
	assert.Contains(t, output, "Total: 0")
	assert.NotContains(t, output, "Automated Plans:")
	assert.NotContains(t, output, "Manual Plans:")
}

func TestWriteTriageJSON(t *testing.T) {
	result := &requirement.TriageResult{
		PolicyID: "test-policy",
		Automated: []requirement.TriagedPlan{
			{
				PlanID:           "plan-1",
				RequirementID:    "REQ-001",
				EvaluationMethod: "conftest",
			},
		},
		Manual: []requirement.TriagedPlan{},
		Counts: requirement.TriageCounts{
			Automated: 1,
			Manual:    0,
			Total:     1,
		},
	}

	var buf bytes.Buffer
	err := writeTriageJSON(&buf, result)
	require.NoError(t, err)

	var parsed requirement.TriageResult
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, "test-policy", parsed.PolicyID)
	require.Len(t, parsed.Automated, 1)
	assert.Equal(t, "plan-1", parsed.Automated[0].PlanID)
	assert.Equal(t, 1, parsed.Counts.Automated)
	assert.Equal(t, 1, parsed.Counts.Total)
}

func TestWriteTriageHuman_DelegatesToText(t *testing.T) {
	result := &requirement.TriageResult{
		PolicyID:  "test-policy",
		Automated: []requirement.TriagedPlan{},
		Manual:    []requirement.TriagedPlan{},
		Counts:    requirement.TriageCounts{},
	}

	var textBuf, humanBuf bytes.Buffer
	err := writeTriageText(&textBuf, result)
	require.NoError(t, err)

	err = writeTriageHuman(&humanBuf, result)
	require.NoError(t, err)

	assert.Equal(t, textBuf.String(), humanBuf.String(),
		"human format should delegate to text for now")
}
