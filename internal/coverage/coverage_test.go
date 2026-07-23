// SPDX-License-Identifier: Apache-2.0

package coverage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"cuelang.org/go/cue"
	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/internal/requirement"
	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubEvaluator implements evaluator.Evaluator for testing.
type stubEvaluator struct {
	id            string
	fileExtension string
	requiredFiles []string
	testResults   *evaluator.TestResults
	testErr       error
}

func (s *stubEvaluator) ID() string                          { return s.id }
func (s *stubEvaluator) FileExtension() string               { return s.fileExtension }
func (s *stubEvaluator) RequiredFiles() []string             { return s.requiredFiles }
func (s *stubEvaluator) Validate(_ string, _ string) []error { return nil }

func (s *stubEvaluator) CheckContract(_ string, _ string, _ cue.Value) ([]evaluator.ContractViolation, error) {
	return nil, nil
}

func (s *stubEvaluator) Test(_ context.Context, _ map[string]string) (*evaluator.TestResults, error) {
	if s.testErr != nil {
		return nil, s.testErr
	}
	return s.testResults, nil
}

func (s *stubEvaluator) Lint(_ string, _ string) ([]evaluator.LintWarning, error) {
	return nil, nil
}

// buildTestPolicy creates a resolved policy with the given controls and assessment plans.
func buildTestPolicy(policyID string, controls []testControl, mode gemara.ModeType) *requirement.ResolvedPolicy {
	var catalogControls []gemara.Control
	var plans []gemara.AssessmentPlan

	for _, c := range controls {
		var reqs []gemara.AssessmentRequirement
		for _, reqID := range c.requirementIDs {
			reqs = append(reqs, gemara.AssessmentRequirement{
				Id: reqID,
			})
			plans = append(plans, gemara.AssessmentPlan{
				Id:            "plan-" + reqID,
				RequirementId: reqID,
				EvaluationMethods: []gemara.AcceptedMethod{
					{Id: "method-1", Mode: mode},
				},
			})
		}
		catalogControls = append(catalogControls, gemara.Control{
			Id:                     c.controlID,
			AssessmentRequirements: reqs,
		})
	}

	policy := gemara.Policy{
		Metadata: gemara.Metadata{Id: policyID},
		Adherence: gemara.Adherence{
			AssessmentPlans: plans,
		},
	}

	catalog := gemara.ControlCatalog{
		Metadata: gemara.Metadata{Id: "test-catalog"},
		Controls: catalogControls,
	}

	set := requirement.NewArtifactSet()
	set.Catalogs["test-catalog"] = &catalog
	set.Policies[policyID] = &policy

	rp, err := requirement.ResolvePolicy(policy, set)
	if err != nil {
		panic("buildTestPolicy: " + err.Error())
	}
	return rp
}

type testControl struct {
	controlID      string
	requirementIDs []string
}

func newOPAEval() *stubEvaluator {
	return &stubEvaluator{
		id:            "opa",
		fileExtension: ".rego",
		requiredFiles: []string{"complytime-mapping.json"},
	}
}

// writeMappingFile creates a complytime-mapping.json in the given directory.
func writeMappingFile(t *testing.T, dir string, entries []MappingEntry) {
	t.Helper()
	mf := MappingFile{
		Version:  "1",
		Mappings: entries,
	}
	data, err := json.Marshal(mf)
	require.NoError(t, err, "marshaling mapping file")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "complytime-mapping.json"), data, 0o600))
}

// writeRegoFile creates a stub .rego file in the given directory.
func writeRegoFile(t *testing.T, dir, name string) {
	t.Helper()
	content := "package " + name + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, name+".rego"), []byte(content), 0o600))
}

func TestRun_FullCoverage(t *testing.T) {
	dir := t.TempDir()

	controls := []testControl{
		{controlID: "CTL-001", requirementIDs: []string{"CTL-001-AR1", "CTL-001-AR2"}},
		{controlID: "CTL-002", requirementIDs: []string{"CTL-002-AR1"}},
	}
	rp := buildTestPolicy("test-policy", controls, gemara.ModeAutomated)

	writeMappingFile(t, dir, []MappingEntry{
		{ID: "ctl_001_ar1", RequirementID: "CTL-001-AR1"},
		{ID: "ctl_001_ar2", RequirementID: "CTL-001-AR2"},
		{ID: "ctl_002_ar1", RequirementID: "CTL-002-AR1"},
	})

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      newOPAEval(),
	})
	require.NoError(t, err)

	assert.Equal(t, "test-policy", report.PolicyID)
	assert.Equal(t, 3, report.Metrics.TotalAutomated)
	assert.Equal(t, 3, report.Metrics.Implemented)
	assert.Equal(t, 0, report.Metrics.Gaps)
	assert.Equal(t, 100.0, report.Metrics.CoveragePercent)
	assert.Empty(t, report.Warnings)

	for _, entry := range report.Requirements {
		assert.Equal(t, StatusImplemented, entry.Status,
			"requirement %s should be implemented", entry.RequirementID)
	}
}

func TestRun_PartialCoverage(t *testing.T) {
	dir := t.TempDir()

	controls := []testControl{
		{controlID: "CTL-001", requirementIDs: []string{"CTL-001-AR1", "CTL-001-AR2"}},
		{controlID: "CTL-002", requirementIDs: []string{"CTL-002-AR1", "CTL-002-AR2", "CTL-002-AR3"}},
	}
	rp := buildTestPolicy("test-policy", controls, gemara.ModeAutomated)

	writeMappingFile(t, dir, []MappingEntry{
		{ID: "ctl_001_ar1", RequirementID: "CTL-001-AR1"},
		{ID: "ctl_001_ar2", RequirementID: "CTL-001-AR2"},
		{ID: "ctl_002_ar1", RequirementID: "CTL-002-AR1"},
	})

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      newOPAEval(),
	})
	require.NoError(t, err)

	assert.Equal(t, 5, report.Metrics.TotalAutomated)
	assert.Equal(t, 3, report.Metrics.Implemented)
	assert.Equal(t, 2, report.Metrics.Gaps)
	assert.Equal(t, 60.0, report.Metrics.CoveragePercent)

	gapCount := 0
	for _, entry := range report.Requirements {
		if entry.Status == StatusGap {
			gapCount++
		}
	}
	assert.Equal(t, 2, gapCount, "should have 2 gap entries")
}

func TestRun_ZeroCoverage(t *testing.T) {
	dir := t.TempDir()

	controls := []testControl{
		{controlID: "CTL-001", requirementIDs: []string{"CTL-001-AR1", "CTL-001-AR2"}},
		{controlID: "CTL-002", requirementIDs: []string{"CTL-002-AR1", "CTL-002-AR2"}},
	}
	rp := buildTestPolicy("test-policy", controls, gemara.ModeAutomated)

	writeMappingFile(t, dir, []MappingEntry{})

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      newOPAEval(),
	})
	require.NoError(t, err)

	assert.Equal(t, 0, report.Metrics.Implemented)
	assert.Equal(t, 4, report.Metrics.Gaps)
	assert.Equal(t, 0.0, report.Metrics.CoveragePercent)
}

func TestRun_NoMappingFile_Fallback(t *testing.T) {
	dir := t.TempDir()

	controls := []testControl{
		{controlID: "CTL-001", requirementIDs: []string{"CTL-001-AR1"}},
	}
	rp := buildTestPolicy("test-policy", controls, gemara.ModeAutomated)

	writeRegoFile(t, dir, "policy1")
	writeRegoFile(t, dir, "policy2")

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      newOPAEval(),
	})
	require.NoError(t, err)

	assert.Equal(t, 1, report.Metrics.Gaps, "all requirements should be gaps in fallback")
	require.NotEmpty(t, report.Warnings, "should include fallback warnings")
	assert.Contains(t, report.Warnings[0].Message, "mapping file not found")
	assert.Contains(t, report.Warnings[1].Message, "reduced detection precision")
}

func TestRun_MappingEntryNoMatchingRequirement(t *testing.T) {
	dir := t.TempDir()

	controls := []testControl{
		{controlID: "CTL-001", requirementIDs: []string{"CTL-001-AR1"}},
	}
	rp := buildTestPolicy("test-policy", controls, gemara.ModeAutomated)

	writeMappingFile(t, dir, []MappingEntry{
		{ID: "ctl_001_ar1", RequirementID: "CTL-001-AR1"},
		{ID: "ctl_extra", RequirementID: "CTL-EXTRA-001-AR1"},
	})

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      newOPAEval(),
	})
	require.NoError(t, err)

	assert.Equal(t, 1, report.Metrics.TotalAutomated)
	assert.Equal(t, 1, report.Metrics.Implemented)
	assert.Equal(t, 0, report.Metrics.Gaps)
}

func TestRun_AllManualRequirements(t *testing.T) {
	dir := t.TempDir()

	controls := []testControl{
		{controlID: "CTL-001", requirementIDs: []string{"CTL-001-AR1", "CTL-001-AR2"}},
		{controlID: "CTL-002", requirementIDs: []string{"CTL-002-AR1"}},
	}
	rp := buildTestPolicy("test-policy", controls, gemara.ModeManual)

	writeMappingFile(t, dir, []MappingEntry{})

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      newOPAEval(),
	})
	require.NoError(t, err)

	assert.Equal(t, 0, report.Metrics.TotalAutomated)
	assert.Equal(t, 0.0, report.Metrics.CoveragePercent)
	assert.Len(t, report.Manual, 3)
	assert.Empty(t, report.Requirements, "all manual — no automated requirements")
}

func TestRun_TestEnrichment_Passing(t *testing.T) {
	dir := t.TempDir()

	controls := []testControl{
		{controlID: "CTL-001", requirementIDs: []string{"CTL-001-AR1"}},
	}
	rp := buildTestPolicy("test-policy", controls, gemara.ModeAutomated)

	writeMappingFile(t, dir, []MappingEntry{
		{ID: "ctl_001_ar1", RequirementID: "CTL-001-AR1"},
	})
	writeRegoFile(t, dir, "ctl_001_ar1")

	eval := newOPAEval()
	eval.testResults = &evaluator.TestResults{
		Total: 1, Passed: 1, Failed: 0,
	}

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      eval,
		RunTests:       true,
	})
	require.NoError(t, err)

	require.Len(t, report.Requirements, 1)
	assert.Equal(t, StatusImplementedPassing, report.Requirements[0].Status)
	assert.Equal(t, 1, report.Metrics.Passing)
}

func TestRun_TestEnrichment_Failing(t *testing.T) {
	dir := t.TempDir()

	controls := []testControl{
		{controlID: "CTL-001", requirementIDs: []string{"CTL-001-AR1"}},
	}
	rp := buildTestPolicy("test-policy", controls, gemara.ModeAutomated)

	writeMappingFile(t, dir, []MappingEntry{
		{ID: "ctl_001_ar1", RequirementID: "CTL-001-AR1"},
	})
	writeRegoFile(t, dir, "ctl_001_ar1")

	eval := newOPAEval()
	eval.testResults = &evaluator.TestResults{
		Total: 1, Passed: 0, Failed: 1,
		Errors: []string{"test_ctl_001_ar1 failed"},
	}

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      eval,
		RunTests:       true,
	})
	require.NoError(t, err)

	require.Len(t, report.Requirements, 1)
	assert.Equal(t, StatusImplementedFailing, report.Requirements[0].Status)
	assert.Equal(t, 1, report.Metrics.Failing)
	assert.Len(t, report.Requirements[0].TestErrors, 1)
}

func TestRun_TestEnrichment_Disabled(t *testing.T) {
	dir := t.TempDir()

	controls := []testControl{
		{controlID: "CTL-001", requirementIDs: []string{"CTL-001-AR1"}},
	}
	rp := buildTestPolicy("test-policy", controls, gemara.ModeAutomated)

	writeMappingFile(t, dir, []MappingEntry{
		{ID: "ctl_001_ar1", RequirementID: "CTL-001-AR1"},
	})

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      newOPAEval(),
		RunTests:       false,
	})
	require.NoError(t, err)

	assert.Equal(t, StatusImplemented, report.Requirements[0].Status,
		"should not have pass/fail enrichment when tests disabled")
	assert.Equal(t, 0, report.Metrics.Passing)
	assert.Equal(t, 0, report.Metrics.Failing)
}

func TestRun_MissingRequiredInputs(t *testing.T) {
	eval := newOPAEval()
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		opts        Options
		errContains string
	}{
		{
			name:        "nil resolved policy",
			opts:        Options{PolicyDir: tmpDir, Evaluator: eval},
			errContains: "resolved policy is required",
		},
		{
			name:        "empty policy dir",
			opts:        Options{ResolvedPolicy: &requirement.ResolvedPolicy{}, Evaluator: eval},
			errContains: "policy directory is required",
		},
		{
			name:        "nil evaluator",
			opts:        Options{ResolvedPolicy: &requirement.ResolvedPolicy{}, PolicyDir: tmpDir},
			errContains: "evaluator is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Run(context.Background(), tc.opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestRun_NonexistentPolicyDir(t *testing.T) {
	eval := newOPAEval()
	rp := buildTestPolicy("test-policy", []testControl{
		{controlID: "CTL-001", requirementIDs: []string{"CTL-001-AR1"}},
	}, gemara.ModeAutomated)

	_, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      filepath.Join(t.TempDir(), "nonexistent"),
		Evaluator:      eval,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy directory")
}

func TestComputeMetrics(t *testing.T) {
	t.Run("mixed statuses", func(t *testing.T) {
		entries := []RequirementEntry{
			{RequirementID: "R1", Status: StatusImplementedPassing},
			{RequirementID: "R2", Status: StatusImplementedFailing},
			{RequirementID: "R3", Status: StatusImplemented},
			{RequirementID: "R4", Status: StatusGap},
			{RequirementID: "R5", Status: StatusGap},
		}

		m := computeMetrics(entries)

		assert.Equal(t, 5, m.TotalAutomated)
		assert.Equal(t, 3, m.Implemented)
		assert.Equal(t, 2, m.Gaps)
		assert.Equal(t, 1, m.Passing)
		assert.Equal(t, 1, m.Failing)
		assert.Equal(t, 60.0, m.CoveragePercent)
	})

	t.Run("empty entries", func(t *testing.T) {
		m := computeMetrics([]RequirementEntry{})

		assert.Equal(t, 0, m.TotalAutomated)
		assert.Equal(t, 0, m.Implemented)
		assert.Equal(t, 0, m.Gaps)
		assert.Equal(t, 0.0, m.CoveragePercent, "should be 0 not NaN for empty entries")
	})
}
