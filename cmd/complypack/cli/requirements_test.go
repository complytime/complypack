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

func TestRequirementsCommand(t *testing.T) {
	root := New()

	cmd, _, err := root.Find([]string{"requirements"})
	require.NoError(t, err, "requirements command should exist")
	assert.Equal(t, "requirements", cmd.Name())
	assert.NotEmpty(t, cmd.Short)

	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("catalog"),
		"should have --catalog flag")
	assert.NotNil(t, flags.Lookup("control"),
		"should have --control flag")
	assert.NotNil(t, flags.Lookup("scope"),
		"should have --scope flag")
	assert.NotNil(t, flags.Lookup("config"),
		"should have --config flag")
	assert.NotNil(t, flags.Lookup("cache-dir"),
		"should have --cache-dir flag")
	assert.NotNil(t, flags.Lookup("format"),
		"should have --format flag")
	assert.NotNil(t, flags.Lookup("source"),
		"should have --source flag")
}

func TestRequirementsCommand_MissingRequiredFlags(t *testing.T) {
	root := New()
	root.SetArgs([]string{"requirements"})

	err := root.Execute()
	assert.Error(t, err,
		"should error when --catalog is missing")
}

func TestWriteRequirementsText(t *testing.T) {
	results := []requirement.AssessmentRequirementInfo{
		{
			ID:            "AR-001",
			ControlID:     "CTL-001",
			Text:          "Requirement text one",
			Applicability: []string{"maturity-1"},
			Parameters:    map[string]string{"key": "val"},
		},
		{
			ID:        "AR-002",
			ControlID: "CTL-002",
			Text:      "Requirement text two",
		},
	}

	var buf bytes.Buffer
	err := writeRequirementsText(&buf, "my-catalog", results)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Requirements: my-catalog")
	assert.Contains(t, output, "Count: 2")
	assert.Contains(t, output, "AR-001")
	assert.Contains(t, output, "CTL-001")
	assert.Contains(t, output, "Requirement text one")
	assert.Contains(t, output, "maturity-1")
	assert.Contains(t, output, "key: val")
	assert.Contains(t, output, "AR-002")
	assert.Contains(t, output, "CTL-002")
}

func TestWriteRequirementsText_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := writeRequirementsText(
		&buf, "empty-catalog",
		[]requirement.AssessmentRequirementInfo{},
	)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Requirements: empty-catalog")
	assert.Contains(t, output, "Count: 0")
}

func TestWriteRequirementsJSON(t *testing.T) {
	results := []requirement.AssessmentRequirementInfo{
		{
			ID:            "AR-001",
			ControlID:     "CTL-001",
			Text:          "Test text",
			Applicability: []string{"scope-1"},
			Parameters:    map[string]string{"p1": "v1"},
		},
	}

	var buf bytes.Buffer
	err := writeRequirementsJSON(&buf, results)
	require.NoError(t, err)

	var parsed []requirement.AssessmentRequirementInfo
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err, "output should be valid JSON")

	require.Len(t, parsed, 1)
	assert.Equal(t, "AR-001", parsed[0].ID)
	assert.Equal(t, "CTL-001", parsed[0].ControlID)
	assert.Equal(t, "Test text", parsed[0].Text)
	assert.Equal(t, []string{"scope-1"}, parsed[0].Applicability)
	assert.Equal(t, "v1", parsed[0].Parameters["p1"])
}

func TestWriteRequirementsJSON_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := writeRequirementsJSON(
		&buf,
		[]requirement.AssessmentRequirementInfo{},
	)
	require.NoError(t, err)

	var parsed []requirement.AssessmentRequirementInfo
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err, "output should be valid JSON")
	assert.Empty(t, parsed)
}

func TestWriteRequirementsHuman_DelegatesToText(t *testing.T) {
	results := []requirement.AssessmentRequirementInfo{
		{
			ID:        "AR-001",
			ControlID: "CTL-001",
			Text:      "Some text",
		},
	}

	var textBuf, humanBuf bytes.Buffer
	err := writeRequirementsText(
		&textBuf, "cat", results,
	)
	require.NoError(t, err)

	err = writeRequirementsHuman(
		&humanBuf, "cat", results,
	)
	require.NoError(t, err)

	assert.Equal(t, textBuf.String(), humanBuf.String(),
		"human format should delegate to text for now")
}
