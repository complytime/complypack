// SPDX-License-Identifier: Apache-2.0

package coverage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/internal/requirement"
	"github.com/complytime/complypack/internal/testresult"
	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers for gap-reporting coverage tests
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Gap-reporting coverage tests
// ---------------------------------------------------------------------------

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

	// Verify aggregate attribution warning is present
	require.NotEmpty(t, report.Warnings, "should warn about aggregate attribution")
	found := false
	for _, w := range report.Warnings {
		if strings.Contains(w.Message, "use per-requirement attribution for granular status") {
			found = true
			break
		}
	}
	assert.True(t, found, "should contain aggregate attribution warning")
}

func TestRun_TestEnrichment_AggregateAttribution(t *testing.T) {
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
	writeRegoFile(t, dir, "ctl_001_ar1")
	writeRegoFile(t, dir, "ctl_001_ar2")
	writeRegoFile(t, dir, "ctl_002_ar1")

	eval := newOPAEval()
	eval.testResults = &evaluator.TestResults{
		Total: 3, Passed: 2, Failed: 1,
		Errors: []string{"test_ctl_002_ar1 failed"},
	}

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      eval,
		RunTests:       true,
	})
	require.NoError(t, err)

	// All 3 implemented requirements should be marked failing (aggregate behavior)
	for _, req := range report.Requirements {
		assert.Equal(t, StatusImplementedFailing, req.Status,
			"requirement %s should be marked failing due to aggregate attribution", req.RequirementID)
	}
	assert.Equal(t, 3, report.Metrics.Failing)
	assert.Equal(t, 0, report.Metrics.Passing)

	// Must include the aggregate attribution warning
	found := false
	for _, w := range report.Warnings {
		if strings.Contains(w.Message, "1 of 3 tests failed") {
			found = true
			break
		}
	}
	assert.True(t, found, "should warn about aggregate attribution with failure counts")
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

// ---------------------------------------------------------------------------
// Per-requirement test attribution tests
// ---------------------------------------------------------------------------

func TestNormalizePackageToRequirement(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple two-level",
			input:    "data.policy.ac_2_1_test",
			expected: "ac-2.1",
		},
		{
			name:     "single-level",
			input:    "data.policy.sc_7_test",
			expected: "sc-7",
		},
		{
			name:     "multi-level",
			input:    "data.policy.ac_2_1_3_test",
			expected: "ac-2.1.3",
		},
		{
			name:     "alpha sub-part",
			input:    "data.policy.ia_5_1_a_test",
			expected: "ia-5.1.a",
		},
		{
			name:     "non-matching pattern",
			input:    "data.policy.container_security_test",
			expected: "",
		},
		{
			name:     "no test suffix",
			input:    "data.policy.ac_2_1",
			expected: "ac-2.1",
		},
		{
			name:     "bare segment",
			input:    "ac_2_1_test",
			expected: "ac-2.1",
		},
		{
			name:     "deeply nested package path",
			input:    "data.policy.kubernetes.ac_2_1_test",
			expected: "ac-2.1",
		},
		{
			name:     "no numeric segments",
			input:    "data.policy.helpers_test",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePackageToRequirement(tt.input)
			assert.Equal(t, tt.expected, got, "NormalizePackageToRequirement(%q)", tt.input)
		})
	}
}

func TestBuildConventionMapping(t *testing.T) {
	t.Run("nil_details", func(t *testing.T) {
		mapping := BuildConventionMapping(nil)
		assert.Empty(t, mapping)
	})

	t.Run("empty_details", func(t *testing.T) {
		mapping := BuildConventionMapping([]testresult.Detail{})
		assert.Empty(t, mapping)
	})

	t.Run("populated_details", func(t *testing.T) {
		details := []testresult.Detail{
			{Package: "data.policy.ac_2_1_test", Name: "test_1", Passed: true},
			{Package: "data.policy.ac_2_1_test", Name: "test_2", Passed: true}, // duplicate package
			{Package: "data.policy.sc_7_test", Name: "test_1", Passed: true},
			{Package: "data.policy.container_security_test", Name: "test_1", Passed: true}, // non-matching
		}

		mapping := BuildConventionMapping(details)

		// Should have 2 mapped packages (ac_2_1 and sc_7), not container_security.
		require.Len(t, mapping, 2)
		assert.Equal(t, []string{"ac-2.1"}, mapping["data.policy.ac_2_1_test"])
		assert.Equal(t, []string{"sc-7"}, mapping["data.policy.sc_7_test"])
	})
}

func TestEnrichWithTestResults(t *testing.T) {
	t.Run("all_pass", func(t *testing.T) {
		mapping := PackageMapping{
			"data.policy.ac_2_1_test": {"ac-2.1"},
		}
		results := &evaluator.TestResults{
			Details: []testresult.Detail{
				{Package: "data.policy.ac_2_1_test", Name: "test_1", Passed: true},
				{Package: "data.policy.ac_2_1_test", Name: "test_2", Passed: true},
			},
		}

		got := EnrichWithTestResults(mapping, results)
		assert.Equal(t, Passing, got["ac-2.1"])
	})

	t.Run("mixed_pass_fail", func(t *testing.T) {
		mapping := PackageMapping{
			"data.policy.ac_2_1_test": {"ac-2.1"},
		}
		results := &evaluator.TestResults{
			Details: []testresult.Detail{
				{Package: "data.policy.ac_2_1_test", Name: "test_1", Passed: true},
				{Package: "data.policy.ac_2_1_test", Name: "test_2", Passed: false, Error: "denied"},
			},
		}

		got := EnrichWithTestResults(mapping, results)
		assert.Equal(t, Failing, got["ac-2.1"])
	})

	t.Run("untested_requirement", func(t *testing.T) {
		mapping := PackageMapping{
			"data.policy.ac_2_1_test": {"ac-2.1"},
			"data.policy.sc_7_test":   {"sc-7"}, // no test details for this package
		}
		results := &evaluator.TestResults{
			Details: []testresult.Detail{
				{Package: "data.policy.ac_2_1_test", Name: "test_1", Passed: true},
			},
		}

		got := EnrichWithTestResults(mapping, results)
		assert.Equal(t, Passing, got["ac-2.1"])
		assert.Equal(t, Untested, got["sc-7"])
	})

	t.Run("multi_package_per_requirement_all_pass", func(t *testing.T) {
		mapping := PackageMapping{
			"data.policy.ac_2_1_test":   {"ac-2.1"},
			"data.policy.ac_2_1_a_test": {"ac-2.1"},
		}
		results := &evaluator.TestResults{
			Details: []testresult.Detail{
				{Package: "data.policy.ac_2_1_test", Name: "test_1", Passed: true},
				{Package: "data.policy.ac_2_1_a_test", Name: "test_1", Passed: true},
			},
		}

		got := EnrichWithTestResults(mapping, results)
		assert.Equal(t, Passing, got["ac-2.1"])
	})

	t.Run("multi_package_per_requirement_mixed", func(t *testing.T) {
		mapping := PackageMapping{
			"data.policy.ac_2_1_test":   {"ac-2.1"},
			"data.policy.ac_2_1_a_test": {"ac-2.1"},
		}
		results := &evaluator.TestResults{
			Details: []testresult.Detail{
				{Package: "data.policy.ac_2_1_test", Name: "test_1", Passed: true},
				{Package: "data.policy.ac_2_1_a_test", Name: "test_1", Passed: false, Error: "failed"},
			},
		}

		got := EnrichWithTestResults(mapping, results)
		assert.Equal(t, Failing, got["ac-2.1"])
	})

	// This mapping is not reachable via BuildConventionMapping (which maps
	// one package to one requirement). It tests EnrichWithTestResults in
	// isolation for multi-value mappings loaded via LoadOverrideMapping.
	t.Run("one_package_maps_to_multiple_requirements", func(t *testing.T) {
		mapping := PackageMapping{
			"data.policy.shared_test": {"ac-2.1", "sc-7"},
		}
		results := &evaluator.TestResults{
			Details: []testresult.Detail{
				{Package: "data.policy.shared_test", Name: "test_1", Passed: true},
			},
		}

		got := EnrichWithTestResults(mapping, results)
		assert.Equal(t, Passing, got["ac-2.1"])
		assert.Equal(t, Passing, got["sc-7"])
	})

	t.Run("nil_mapping", func(t *testing.T) {
		results := &evaluator.TestResults{
			Details: []testresult.Detail{
				{Package: "data.policy.ac_2_1_test", Passed: true},
			},
		}

		got := EnrichWithTestResults(nil, results)
		assert.Empty(t, got)
	})

	t.Run("nil_results", func(t *testing.T) {
		mapping := PackageMapping{
			"data.policy.ac_2_1_test": {"ac-2.1"},
		}

		got := EnrichWithTestResults(mapping, nil)
		assert.Empty(t, got)
	})

	t.Run("empty_details_backward_compat", func(t *testing.T) {
		mapping := PackageMapping{
			"data.policy.ac_2_1_test": {"ac-2.1"},
		}
		results := &evaluator.TestResults{
			Total:  5,
			Passed: 5,
			// Details is nil - backward compatibility case.
		}

		got := EnrichWithTestResults(mapping, results)
		assert.Equal(t, Untested, got["ac-2.1"])
	})

	t.Run("mapping_references_unknown_packages", func(t *testing.T) {
		mapping := PackageMapping{
			"data.policy.ac_2_1_test": {"ac-2.1"},
			"data.policy.nonexistent": {"sc-7"}, // no test details for this
		}
		results := &evaluator.TestResults{
			Details: []testresult.Detail{
				{Package: "data.policy.ac_2_1_test", Passed: true},
			},
		}

		got := EnrichWithTestResults(mapping, results)
		assert.Equal(t, Passing, got["ac-2.1"])
		assert.Equal(t, Untested, got["sc-7"])
	})
}

func TestLoadOverrideMapping(t *testing.T) {
	t.Run("valid_file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "coverage-mapping.yaml")
		content := `version: "1"
mappings:
  "data.policy.custom_check": ["sc-7"]
  "data.policy.another_pkg": ["ac-2.1", "ac-2.2"]
`
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))

		mapping, err := LoadOverrideMapping(path)
		require.NoError(t, err)
		require.Len(t, mapping, 2)
		assert.Equal(t, []string{"sc-7"}, mapping["data.policy.custom_check"])
		assert.Len(t, mapping["data.policy.another_pkg"], 2)
	})

	t.Run("file_not_found", func(t *testing.T) {
		mapping, err := LoadOverrideMapping("/nonexistent/path.yaml")
		require.NoError(t, err)
		assert.Nil(t, mapping)
	})

	t.Run("malformed_yaml", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "coverage-mapping.yaml")
		require.NoError(t, os.WriteFile(path, []byte("{{invalid yaml"), 0600))

		_, err := LoadOverrideMapping(path)
		assert.Error(t, err)
	})

	t.Run("unsupported_version", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "coverage-mapping.yaml")
		content := `version: "2"
mappings:
  "data.policy.custom_check": ["sc-7"]
`
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))

		_, err := LoadOverrideMapping(path)
		assert.Error(t, err)
	})

	t.Run("empty_mappings", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "coverage-mapping.yaml")
		content := `version: "1"
mappings: {}
`
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))

		mapping, err := LoadOverrideMapping(path)
		require.NoError(t, err)
		require.NotNil(t, mapping)
		assert.Empty(t, mapping)
	})
}

func TestAttributeTests(t *testing.T) {
	t.Run("nil_results", func(t *testing.T) {
		got, err := AttributeTests("", "", nil)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("empty_details", func(t *testing.T) {
		results := &evaluator.TestResults{Total: 5, Passed: 5}
		got, err := AttributeTests("", "", results)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("convention_only", func(t *testing.T) {
		results := &evaluator.TestResults{
			Details: []testresult.Detail{
				{Package: "data.policy.ac_2_1_test", Name: "test_1", Passed: true},
				{Package: "data.policy.sc_7_test", Name: "test_1", Passed: false, Error: "denied"},
			},
		}

		got, err := AttributeTests("", "", results)
		require.NoError(t, err)
		assert.Equal(t, Passing, got["ac-2.1"])
		assert.Equal(t, Failing, got["sc-7"])
	})

	t.Run("with_override_file", func(t *testing.T) {
		dir := t.TempDir()
		overridePath := filepath.Join(dir, "coverage-mapping.yaml")
		content := `version: "1"
mappings:
  "data.policy.custom_check": ["sc-7"]
`
		require.NoError(t, os.WriteFile(overridePath, []byte(content), 0600))

		results := &evaluator.TestResults{
			Details: []testresult.Detail{
				{Package: "data.policy.ac_2_1_test", Name: "test_1", Passed: true},
				{Package: "data.policy.custom_check", Name: "test_1", Passed: true},
			},
		}

		got, err := AttributeTests(dir, "", results)
		require.NoError(t, err)
		assert.Equal(t, Passing, got["ac-2.1"], "convention mapping")
		assert.Equal(t, Passing, got["sc-7"], "override mapping")
	})

	t.Run("with_explicit_mapping_path", func(t *testing.T) {
		dir := t.TempDir()
		overridePath := filepath.Join(dir, "custom-mapping.yaml")
		content := `version: "1"
mappings:
  "data.policy.custom_check": ["sc-7"]
`
		require.NoError(t, os.WriteFile(overridePath, []byte(content), 0600))

		results := &evaluator.TestResults{
			Details: []testresult.Detail{
				{Package: "data.policy.custom_check", Name: "test_1", Passed: true},
			},
		}

		got, err := AttributeTests("", overridePath, results)
		require.NoError(t, err)
		assert.Equal(t, Passing, got["sc-7"])
	})

	t.Run("malformed_override_returns_error", func(t *testing.T) {
		dir := t.TempDir()
		overridePath := filepath.Join(dir, "coverage-mapping.yaml")
		require.NoError(t, os.WriteFile(overridePath, []byte("{{invalid"), 0600))

		results := &evaluator.TestResults{
			Details: []testresult.Detail{
				{Package: "data.policy.ac_2_1_test", Passed: true},
			},
		}

		_, err := AttributeTests(dir, "", results)
		assert.Error(t, err)
	})

	t.Run("override_takes_precedence", func(t *testing.T) {
		dir := t.TempDir()
		overridePath := filepath.Join(dir, "coverage-mapping.yaml")
		// Override maps ac_2_1_test to a different requirement.
		content := `version: "1"
mappings:
  "data.policy.ac_2_1_test": ["custom-req"]
`
		require.NoError(t, os.WriteFile(overridePath, []byte(content), 0600))

		results := &evaluator.TestResults{
			Details: []testresult.Detail{
				{Package: "data.policy.ac_2_1_test", Name: "test_1", Passed: true},
			},
		}

		got, err := AttributeTests(dir, "", results)
		require.NoError(t, err)
		// Override replaces convention mapping.
		assert.Equal(t, Passing, got["custom-req"], "override mapping")
		// Convention mapping should be overridden.
		assert.NotContains(t, got, "ac-2.1", "convention should be overridden")
	})
}
