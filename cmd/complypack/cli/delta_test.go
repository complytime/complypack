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

func TestDeltaCommand(t *testing.T) {
	root := New()

	cmd, _, err := root.Find([]string{"delta"})
	require.NoError(t, err, "delta command should exist")
	assert.Equal(t, "delta", cmd.Name())
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

	// delta must NOT have --catalog
	assert.Nil(t, flags.Lookup("catalog"),
		"delta should not have --catalog flag")
}

func TestDeltaCommand_MissingRequiredFlags(t *testing.T) {
	root := New()
	root.SetArgs([]string{"delta"})

	err := root.Execute()
	assert.Error(t, err,
		"should error when --policy is missing")
}

func TestWriteDeltaText(t *testing.T) {
	report := &requirement.DeltaReport{
		PolicyID:         "test-policy",
		CatalogsCompared: []string{"catalog-a", "catalog-b"},
		Comparisons: []requirement.ParameterComparison{
			{
				RequirementID:   "REQ-001",
				Label:           "tls_version",
				PolicyValue:     "1.2",
				PolicySource:    "test-policy",
				RequirementText: "Must use TLS",
				CatalogSource:   "catalog-a",
			},
		},
	}

	var buf bytes.Buffer
	err := writeDeltaText(&buf, report)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Delta: test-policy")
	assert.Contains(t, output, "Catalogs compared: 2")
	assert.Contains(t, output, "Comparisons: 1")
	assert.Contains(t, output, "REQ-001")
	assert.Contains(t, output, "tls_version")
	assert.Contains(t, output, "Policy value: 1.2")
	assert.Contains(t, output, "Requirement: Must use TLS")
}

func TestWriteDeltaText_NoComparisons(t *testing.T) {
	report := &requirement.DeltaReport{
		PolicyID:         "empty-policy",
		CatalogsCompared: []string{},
		Comparisons:      []requirement.ParameterComparison{},
	}

	var buf bytes.Buffer
	err := writeDeltaText(&buf, report)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Delta: empty-policy")
	assert.Contains(t, output, "Catalogs compared: 0")
	assert.Contains(t, output, "Comparisons: 0")
}

func TestWriteDeltaText_NoRequirementText(t *testing.T) {
	report := &requirement.DeltaReport{
		PolicyID:         "test-policy",
		CatalogsCompared: []string{"catalog-a"},
		Comparisons: []requirement.ParameterComparison{
			{
				RequirementID: "REQ-001",
				Label:         "param",
				PolicyValue:   "val",
			},
		},
	}

	var buf bytes.Buffer
	err := writeDeltaText(&buf, report)
	require.NoError(t, err)

	output := buf.String()
	assert.NotContains(t, output, "Requirement:")
}

func TestWriteDeltaJSON(t *testing.T) {
	report := &requirement.DeltaReport{
		PolicyID:         "test-policy",
		CatalogsCompared: []string{"cat-1"},
		Comparisons: []requirement.ParameterComparison{
			{
				RequirementID:   "REQ-001",
				Label:           "key",
				PolicyValue:     "val",
				PolicySource:    "test-policy",
				RequirementText: "req text",
				CatalogSource:   "cat-1",
			},
		},
	}

	var buf bytes.Buffer
	err := writeDeltaJSON(&buf, report)
	require.NoError(t, err)

	var parsed requirement.DeltaReport
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, "test-policy", parsed.PolicyID)
	require.Len(t, parsed.CatalogsCompared, 1)
	require.Len(t, parsed.Comparisons, 1)
	assert.Equal(t, "REQ-001",
		parsed.Comparisons[0].RequirementID)
	assert.Equal(t, "val",
		parsed.Comparisons[0].PolicyValue)
}

func TestWriteDeltaHuman_DelegatesToText(t *testing.T) {
	report := &requirement.DeltaReport{
		PolicyID:         "test-policy",
		CatalogsCompared: []string{},
		Comparisons:      []requirement.ParameterComparison{},
	}

	var textBuf, humanBuf bytes.Buffer
	err := writeDeltaText(&textBuf, report)
	require.NoError(t, err)

	err = writeDeltaHuman(&humanBuf, report)
	require.NoError(t, err)

	assert.Equal(t, textBuf.String(), humanBuf.String(),
		"human format should delegate to text for now")
}
