// SPDX-License-Identifier: Apache-2.0

package requirement_test

import (
	"testing"

	"github.com/complytime/complypack/internal/requirement"
	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
)

func assessmentTestResolvedPolicy() *requirement.ResolvedPolicy {
	catalog := &gemara.ControlCatalog{
		Metadata: gemara.Metadata{Id: "test-catalog"},
		Controls: []gemara.Control{
			{
				Id: "TEST-001",
				AssessmentRequirements: []gemara.AssessmentRequirement{
					{
						Id:            "TEST-001-AR1",
						Text:          "Test requirement",
						Applicability: []string{"maturity-1", "maturity-2", "maturity-3"},
					},
					{
						Id:            "TEST-001-AR2",
						Text:          "Second requirement",
						Applicability: []string{"maturity-2", "maturity-3"},
					},
				},
			},
			{
				Id: "TEST-002",
				AssessmentRequirements: []gemara.AssessmentRequirement{
					{
						Id:            "TEST-002-AR1",
						Text:          "Third requirement",
						Applicability: []string{"maturity-3"},
					},
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
		Adherence: gemara.Adherence{
			AssessmentPlans: []gemara.AssessmentPlan{
				{
					RequirementId: "TEST-001-AR1",
					Parameters: []gemara.Parameter{
						{Label: "threshold", AcceptedValues: []string{"90"}},
					},
				},
			},
		},
	}

	set := &requirement.ArtifactSet{
		Catalogs: map[string]*gemara.ControlCatalog{"test-catalog": catalog},
		Policies: map[string]*gemara.Policy{"test-policy": policy},
		Guidance: make(map[string]*gemara.GuidanceCatalog),
	}

	rp, err := requirement.ResolvePolicy(*policy, set)
	if err != nil {
		panic(err)
	}
	return rp
}

func TestExtractAssessmentRequirements(t *testing.T) {
	rp := assessmentTestResolvedPolicy()

	t.Run("extract all", func(t *testing.T) {
		results := requirement.ExtractAssessmentRequirements(rp, "", nil)
		assert.Len(t, results, 3)
	})

	t.Run("filter by control", func(t *testing.T) {
		results := requirement.ExtractAssessmentRequirements(rp, "TEST-001", nil)
		assert.Len(t, results, 2)
		assert.Equal(t, "TEST-001", results[0].ControlID)
		assert.Equal(t, "TEST-001", results[1].ControlID)
	})

	t.Run("parameters populated from assessment plans", func(t *testing.T) {
		results := requirement.ExtractAssessmentRequirements(rp, "TEST-001", nil)
		assert.Equal(t, "90", results[0].Parameters["threshold"])
		assert.Empty(t, results[1].Parameters)
	})

	t.Run("filter by scope", func(t *testing.T) {
		results := requirement.ExtractAssessmentRequirements(
			rp, "", []string{"maturity-2"},
		)
		assert.Len(t, results, 2)
		for _, r := range results {
			assert.Contains(t, r.Applicability, "maturity-2")
		}
	})

	t.Run("filter by multiple scope values", func(t *testing.T) {
		results := requirement.ExtractAssessmentRequirements(
			rp, "", []string{"maturity-1", "maturity-3"},
		)
		assert.Len(t, results, 3)
	})

	t.Run("filter by scope and control", func(t *testing.T) {
		results := requirement.ExtractAssessmentRequirements(
			rp, "TEST-001", []string{"maturity-2"},
		)
		assert.Len(t, results, 2)
	})

	t.Run("scope filters out non-matching", func(t *testing.T) {
		results := requirement.ExtractAssessmentRequirements(
			rp, "", []string{"maturity-1"},
		)
		assert.Len(t, results, 1)
	})

	t.Run("nil scope returns all", func(t *testing.T) {
		results := requirement.ExtractAssessmentRequirements(rp, "", nil)
		assert.Len(t, results, 3)
	})
}
