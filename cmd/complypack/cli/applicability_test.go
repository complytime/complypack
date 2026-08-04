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

func TestApplicabilityCommand(t *testing.T) {
	root := New()

	cmd, _, err := root.Find([]string{"applicability"})
	require.NoError(t, err,
		"applicability command should exist")
	assert.Equal(t, "applicability", cmd.Name())
	assert.NotEmpty(t, cmd.Short)

	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("catalog"),
		"should have --catalog flag")
	assert.NotNil(t, flags.Lookup("requirement"),
		"should have --requirement flag")
	assert.NotNil(t, flags.Lookup("config"),
		"should have --config flag")
	assert.NotNil(t, flags.Lookup("cache-dir"),
		"should have --cache-dir flag")
	assert.NotNil(t, flags.Lookup("format"),
		"should have --format flag")
	assert.NotNil(t, flags.Lookup("source"),
		"should have --source flag")

	// applicability must NOT have --policy
	assert.Nil(t, flags.Lookup("policy"),
		"applicability should not have --policy flag")
}

func TestApplicabilityCommand_MissingRequiredFlags(t *testing.T) {
	root := New()
	root.SetArgs([]string{"applicability"})

	err := root.Execute()
	assert.Error(t, err,
		"should error when --catalog is missing")
}

func TestWriteApplicabilityText(t *testing.T) {
	result := &requirement.ApplicabilityGroupResult{
		Groups: []requirement.ApplicabilityGroupInfo{
			{
				ID:             "maturity-1",
				Title:          "Maturity Level 1",
				Description:    "Basic requirements",
				RequirementIDs: []string{"REQ-001", "REQ-002"},
			},
			{
				ID:             "maturity-2",
				Title:          "Maturity Level 2",
				RequirementIDs: []string{"REQ-003"},
			},
		},
		Ungrouped: []string{"REQ-099"},
	}

	var buf bytes.Buffer
	err := writeApplicabilityText(
		&buf, "my-catalog", result,
	)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Applicability: my-catalog")
	assert.Contains(t, output, "Groups: 2")
	assert.Contains(t, output, "Ungrouped: 1")
	assert.Contains(t, output, "maturity-1")
	assert.Contains(t, output, "Maturity Level 1")
	assert.Contains(t, output, "Basic requirements")
	assert.Contains(t, output, "REQ-001, REQ-002")
	assert.Contains(t, output, "maturity-2")
	assert.Contains(t, output, "REQ-003")
	assert.Contains(t, output, "REQ-099")
}

func TestWriteApplicabilityText_NoGroups(t *testing.T) {
	result := &requirement.ApplicabilityGroupResult{
		Groups:    []requirement.ApplicabilityGroupInfo{},
		Ungrouped: []string{},
	}

	var buf bytes.Buffer
	err := writeApplicabilityText(
		&buf, "empty-catalog", result,
	)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Applicability: empty-catalog")
	assert.Contains(t, output, "Groups: 0")
	assert.Contains(t, output, "Ungrouped: 0")
}

func TestWriteApplicabilityText_GroupWithoutDescription(
	t *testing.T,
) {
	result := &requirement.ApplicabilityGroupResult{
		Groups: []requirement.ApplicabilityGroupInfo{
			{
				ID:             "basic",
				Title:          "",
				Description:    "",
				RequirementIDs: []string{"REQ-001"},
			},
		},
		Ungrouped: []string{},
	}

	var buf bytes.Buffer
	err := writeApplicabilityText(
		&buf, "test-catalog", result,
	)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "basic")
	assert.Contains(t, output, "REQ-001")
	// Title and description are empty, should not appear
	assert.NotContains(t, output, " - \n")
}

func TestWriteApplicabilityText_GroupWithEmptyRequirements(
	t *testing.T,
) {
	result := &requirement.ApplicabilityGroupResult{
		Groups: []requirement.ApplicabilityGroupInfo{
			{
				ID:             "empty-group",
				Title:          "Empty Group",
				RequirementIDs: []string{},
			},
		},
		Ungrouped: []string{},
	}

	var buf bytes.Buffer
	err := writeApplicabilityText(
		&buf, "test-catalog", result,
	)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "empty-group")
	assert.Contains(t, output, "Empty Group")
	assert.NotContains(t, output, "Requirements:")
}

func TestWriteApplicabilityJSON(t *testing.T) {
	result := &requirement.ApplicabilityGroupResult{
		Groups: []requirement.ApplicabilityGroupInfo{
			{
				ID:             "group-1",
				Title:          "Group One",
				Description:    "First group",
				RequirementIDs: []string{"R1", "R2"},
			},
		},
		Ungrouped: []string{"R3"},
	}

	var buf bytes.Buffer
	err := writeApplicabilityJSON(&buf, result)
	require.NoError(t, err)

	var parsed requirement.ApplicabilityGroupResult
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err, "output should be valid JSON")

	require.Len(t, parsed.Groups, 1)
	assert.Equal(t, "group-1", parsed.Groups[0].ID)
	assert.Equal(t, "Group One", parsed.Groups[0].Title)
	assert.Equal(t, []string{"R1", "R2"},
		parsed.Groups[0].RequirementIDs)
	assert.Equal(t, []string{"R3"}, parsed.Ungrouped)
}

func TestWriteApplicabilityHuman_DelegatesToText(
	t *testing.T,
) {
	result := &requirement.ApplicabilityGroupResult{
		Groups:    []requirement.ApplicabilityGroupInfo{},
		Ungrouped: []string{},
	}

	var textBuf, humanBuf bytes.Buffer
	err := writeApplicabilityText(
		&textBuf, "cat", result,
	)
	require.NoError(t, err)

	err = writeApplicabilityHuman(
		&humanBuf, "cat", result,
	)
	require.NoError(t, err)

	assert.Equal(t, textBuf.String(), humanBuf.String(),
		"human format should delegate to text for now")
}
