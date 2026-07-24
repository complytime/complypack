// SPDX-License-Identifier: Apache-2.0

package requirement

import (
	"testing"

	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func multiControlArtifactSet() *ArtifactSet {
	catalog := &gemara.ControlCatalog{
		Metadata: gemara.Metadata{Id: "test-catalog"},
		Controls: []gemara.Control{
			{
				Id:    "CTRL-001",
				Title: "Input Validation",
				AssessmentRequirements: []gemara.AssessmentRequirement{
					{Id: "REQ-001", Text: "Validate inputs", Applicability: []string{"maturity-1"}},
					{Id: "REQ-002", Text: "Sanitize inputs", Applicability: []string{"maturity-2"}},
				},
			},
			{
				Id:    "CTRL-002",
				Title: "Access Control",
				AssessmentRequirements: []gemara.AssessmentRequirement{
					{Id: "REQ-003", Text: "Enforce RBAC", Applicability: []string{"maturity-1", "maturity-2"}},
				},
			},
			{
				Id:    "CTRL-003",
				Title: "Audit Logging",
				AssessmentRequirements: []gemara.AssessmentRequirement{
					{Id: "REQ-004", Text: "Log access events", Applicability: []string{"maturity-2"}},
					{Id: "REQ-005", Text: "Log admin actions", Applicability: []string{"maturity-2"}},
					{Id: "REQ-006", Text: "Retain logs", Applicability: []string{"maturity-1"}},
				},
			},
		},
	}

	policy := &gemara.Policy{
		Metadata: gemara.Metadata{
			Id: "test-policy",
			MappingReferences: []gemara.MappingReference{
				{Id: "test-catalog"},
			},
		},
		Imports: gemara.Imports{
			Catalogs: []gemara.CatalogImport{
				{ReferenceId: "test-catalog"},
			},
		},
	}

	return &ArtifactSet{
		Catalogs: map[string]*gemara.ControlCatalog{"test-catalog": catalog},
		Policies: map[string]*gemara.Policy{"test-policy": policy},
		Guidance: make(map[string]*gemara.GuidanceCatalog),
		Mappings: make(map[string]*gemara.MappingDocument),
	}
}

func TestControlSummaries_Unfiltered(t *testing.T) {
	set := multiControlArtifactSet()
	policy := set.Policies["test-policy"]

	rp, err := ResolvePolicy(*policy, set)
	require.NoError(t, err)

	summary, controls := rp.ControlSummaries(rp.ControlIDs(), nil)

	assert.Equal(t, 3, summary.TotalControls)
	assert.Equal(t, 6, summary.TotalRequirements)
	assert.Len(t, controls, 3)

	controlMap := make(map[string]ControlSummary)
	for _, c := range controls {
		controlMap[c.ID] = c
	}

	assert.Equal(t, "Input Validation", controlMap["CTRL-001"].Title)
	assert.Equal(t, 2, controlMap["CTRL-001"].RequirementCount)

	assert.Equal(t, "Access Control", controlMap["CTRL-002"].Title)
	assert.Equal(t, 1, controlMap["CTRL-002"].RequirementCount)

	assert.Equal(t, "Audit Logging", controlMap["CTRL-003"].Title)
	assert.Equal(t, 3, controlMap["CTRL-003"].RequirementCount)
}

func TestControlSummaries_ScopeFiltered(t *testing.T) {
	set := multiControlArtifactSet()
	policy := set.Policies["test-policy"]

	rp, err := ResolvePolicy(*policy, set)
	require.NoError(t, err)

	summary, controls := rp.ControlSummaries(rp.ControlIDs(), []string{"maturity-1"})

	// maturity-1: REQ-001 (CTRL-001), REQ-003 (CTRL-002), REQ-006 (CTRL-003)
	assert.Equal(t, 3, summary.TotalControls)
	assert.Equal(t, 3, summary.TotalRequirements)

	controlMap := make(map[string]ControlSummary)
	for _, c := range controls {
		controlMap[c.ID] = c
	}

	assert.Equal(t, 1, controlMap["CTRL-001"].RequirementCount)
	assert.Equal(t, 1, controlMap["CTRL-002"].RequirementCount)
	assert.Equal(t, 1, controlMap["CTRL-003"].RequirementCount)
}

func TestControlSummaries_ScopeFiltered_ExcludesZeroCount(t *testing.T) {
	set := multiControlArtifactSet()
	// Make CTRL-002 only have maturity-2 requirements
	set.Catalogs["test-catalog"].Controls[1].AssessmentRequirements = []gemara.AssessmentRequirement{
		{Id: "REQ-003", Text: "Enforce RBAC", Applicability: []string{"maturity-2"}},
	}
	policy := set.Policies["test-policy"]

	rp, err := ResolvePolicy(*policy, set)
	require.NoError(t, err)

	summary, controls := rp.ControlSummaries(rp.ControlIDs(), []string{"maturity-1"})

	// maturity-1: REQ-001 (CTRL-001), REQ-006 (CTRL-003) — CTRL-002 excluded
	assert.Equal(t, 2, summary.TotalControls)
	assert.Equal(t, 2, summary.TotalRequirements)
	assert.Len(t, controls, 2)

	for _, c := range controls {
		assert.NotEqual(t, "CTRL-002", c.ID)
	}
}

func TestControlSummaries_SingleControl(t *testing.T) {
	set := multiControlArtifactSet()
	policy := set.Policies["test-policy"]

	rp, err := ResolvePolicy(*policy, set)
	require.NoError(t, err)

	summary, controls := rp.ControlSummaries([]string{"CTRL-003"}, nil)

	assert.Equal(t, 1, summary.TotalControls)
	assert.Equal(t, 3, summary.TotalRequirements)
	assert.Len(t, controls, 1)
	assert.Equal(t, "CTRL-003", controls[0].ID)
	assert.Equal(t, "Audit Logging", controls[0].Title)
	assert.Equal(t, 3, controls[0].RequirementCount)
}

func TestControlSummaries_SingleControlWithScope(t *testing.T) {
	set := multiControlArtifactSet()
	policy := set.Policies["test-policy"]

	rp, err := ResolvePolicy(*policy, set)
	require.NoError(t, err)

	summary, controls := rp.ControlSummaries([]string{"CTRL-003"}, []string{"maturity-2"})

	// CTRL-003 maturity-2: REQ-004, REQ-005
	assert.Equal(t, 1, summary.TotalControls)
	assert.Equal(t, 2, summary.TotalRequirements)
	assert.Len(t, controls, 1)
	assert.Equal(t, 2, controls[0].RequirementCount)
}

func TestControlByID(t *testing.T) {
	set := multiControlArtifactSet()
	policy := set.Policies["test-policy"]

	rp, err := ResolvePolicy(*policy, set)
	require.NoError(t, err)

	t.Run("existing control", func(t *testing.T) {
		ctrl := rp.ControlByID("CTRL-001")
		require.NotNil(t, ctrl)
		assert.Equal(t, "Input Validation", ctrl.Title)
	})

	t.Run("unknown control", func(t *testing.T) {
		ctrl := rp.ControlByID("UNKNOWN")
		assert.Nil(t, ctrl)
	})
}
