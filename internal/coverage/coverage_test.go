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
)

// stubEvaluator implements evaluator.Evaluator for testing.
type stubEvaluator struct {
	id            string
	fileExtension string
	requiredFiles []string
	testResults   *evaluator.TestResults
	testErr       error
}

func (s *stubEvaluator) ID() string                  { return s.id }
func (s *stubEvaluator) FileExtension() string        { return s.fileExtension }
func (s *stubEvaluator) RequiredFiles() []string       { return s.requiredFiles }
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

// writeMappingFile creates a complytime-mapping.json in the given directory.
func writeMappingFile(t *testing.T, dir string, entries []MappingEntry) {
	t.Helper()
	mf := MappingFile{
		Version:  "1",
		Mappings: entries,
	}
	data, err := json.Marshal(mf)
	if err != nil {
		t.Fatalf("marshaling mapping file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "complytime-mapping.json"), data, 0o644); err != nil {
		t.Fatalf("writing mapping file: %v", err)
	}
}

// writeRegoFile creates a stub .rego file in the given directory.
func writeRegoFile(t *testing.T, dir, name string) {
	t.Helper()
	content := "package " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, name+".rego"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing rego file: %v", err)
	}
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

	eval := &stubEvaluator{
		id:            "opa",
		fileExtension: ".rego",
		requiredFiles: []string{"complytime-mapping.json"},
	}

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      eval,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if report.PolicyID != "test-policy" {
		t.Errorf("PolicyID = %q, want %q", report.PolicyID, "test-policy")
	}
	if report.Metrics.TotalAutomated != 3 {
		t.Errorf("TotalAutomated = %d, want 3", report.Metrics.TotalAutomated)
	}
	if report.Metrics.Implemented != 3 {
		t.Errorf("Implemented = %d, want 3", report.Metrics.Implemented)
	}
	if report.Metrics.Gaps != 0 {
		t.Errorf("Gaps = %d, want 0", report.Metrics.Gaps)
	}
	if report.Metrics.CoveragePercent != 100 {
		t.Errorf("CoveragePercent = %f, want 100", report.Metrics.CoveragePercent)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("Warnings = %d, want 0", len(report.Warnings))
	}

	for _, entry := range report.Requirements {
		if entry.Status != StatusImplemented {
			t.Errorf("requirement %s status = %q, want %q", entry.RequirementID, entry.Status, StatusImplemented)
		}
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

	eval := &stubEvaluator{
		id:            "opa",
		fileExtension: ".rego",
		requiredFiles: []string{"complytime-mapping.json"},
	}

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      eval,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if report.Metrics.TotalAutomated != 5 {
		t.Errorf("TotalAutomated = %d, want 5", report.Metrics.TotalAutomated)
	}
	if report.Metrics.Implemented != 3 {
		t.Errorf("Implemented = %d, want 3", report.Metrics.Implemented)
	}
	if report.Metrics.Gaps != 2 {
		t.Errorf("Gaps = %d, want 2", report.Metrics.Gaps)
	}
	if report.Metrics.CoveragePercent != 60 {
		t.Errorf("CoveragePercent = %f, want 60", report.Metrics.CoveragePercent)
	}

	// Check that gap entries have the correct status
	gapCount := 0
	for _, entry := range report.Requirements {
		if entry.Status == StatusGap {
			gapCount++
		}
	}
	if gapCount != 2 {
		t.Errorf("gap count = %d, want 2", gapCount)
	}
}

func TestRun_ZeroCoverage(t *testing.T) {
	dir := t.TempDir()

	controls := []testControl{
		{controlID: "CTL-001", requirementIDs: []string{"CTL-001-AR1", "CTL-001-AR2"}},
		{controlID: "CTL-002", requirementIDs: []string{"CTL-002-AR1", "CTL-002-AR2"}},
	}
	rp := buildTestPolicy("test-policy", controls, gemara.ModeAutomated)

	// Empty mapping file - no policies implemented
	writeMappingFile(t, dir, []MappingEntry{})

	eval := &stubEvaluator{
		id:            "opa",
		fileExtension: ".rego",
		requiredFiles: []string{"complytime-mapping.json"},
	}

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      eval,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if report.Metrics.Implemented != 0 {
		t.Errorf("Implemented = %d, want 0", report.Metrics.Implemented)
	}
	if report.Metrics.Gaps != 4 {
		t.Errorf("Gaps = %d, want 4", report.Metrics.Gaps)
	}
	if report.Metrics.CoveragePercent != 0 {
		t.Errorf("CoveragePercent = %f, want 0", report.Metrics.CoveragePercent)
	}
}

func TestRun_NoMappingFile_Fallback(t *testing.T) {
	dir := t.TempDir()

	controls := []testControl{
		{controlID: "CTL-001", requirementIDs: []string{"CTL-001-AR1"}},
	}
	rp := buildTestPolicy("test-policy", controls, gemara.ModeAutomated)

	// Write rego files but no mapping file
	writeRegoFile(t, dir, "policy1")
	writeRegoFile(t, dir, "policy2")

	eval := &stubEvaluator{
		id:            "opa",
		fileExtension: ".rego",
		requiredFiles: []string{"complytime-mapping.json"},
	}

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      eval,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// All requirements should be gaps since we can't determine mapping
	if report.Metrics.Gaps != 1 {
		t.Errorf("Gaps = %d, want 1", report.Metrics.Gaps)
	}
	if len(report.Warnings) == 0 {
		t.Error("expected warnings about fallback scanning, got none")
	}
}

func TestRun_MappingEntryNoMatchingRequirement(t *testing.T) {
	dir := t.TempDir()

	controls := []testControl{
		{controlID: "CTL-001", requirementIDs: []string{"CTL-001-AR1"}},
	}
	rp := buildTestPolicy("test-policy", controls, gemara.ModeAutomated)

	// Mapping file has an extra entry not in the policy
	writeMappingFile(t, dir, []MappingEntry{
		{ID: "ctl_001_ar1", RequirementID: "CTL-001-AR1"},
		{ID: "ctl_extra", RequirementID: "CTL-EXTRA-001-AR1"},
	})

	eval := &stubEvaluator{
		id:            "opa",
		fileExtension: ".rego",
		requiredFiles: []string{"complytime-mapping.json"},
	}

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      eval,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Only the matching requirement should be counted
	if report.Metrics.TotalAutomated != 1 {
		t.Errorf("TotalAutomated = %d, want 1", report.Metrics.TotalAutomated)
	}
	if report.Metrics.Implemented != 1 {
		t.Errorf("Implemented = %d, want 1", report.Metrics.Implemented)
	}
	if report.Metrics.Gaps != 0 {
		t.Errorf("Gaps = %d, want 0", report.Metrics.Gaps)
	}
}

func TestRun_AllManualRequirements(t *testing.T) {
	dir := t.TempDir()

	controls := []testControl{
		{controlID: "CTL-001", requirementIDs: []string{"CTL-001-AR1", "CTL-001-AR2"}},
		{controlID: "CTL-002", requirementIDs: []string{"CTL-002-AR1"}},
	}
	rp := buildTestPolicy("test-policy", controls, gemara.ModeManual)

	writeMappingFile(t, dir, []MappingEntry{})

	eval := &stubEvaluator{
		id:            "opa",
		fileExtension: ".rego",
		requiredFiles: []string{"complytime-mapping.json"},
	}

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      eval,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if report.Metrics.TotalAutomated != 0 {
		t.Errorf("TotalAutomated = %d, want 0", report.Metrics.TotalAutomated)
	}
	if report.Metrics.CoveragePercent != 0 {
		t.Errorf("CoveragePercent = %f, want 0", report.Metrics.CoveragePercent)
	}
	if len(report.Manual) != 3 {
		t.Errorf("Manual count = %d, want 3", len(report.Manual))
	}
	if len(report.Requirements) != 0 {
		t.Errorf("Requirements count = %d, want 0 (all manual)", len(report.Requirements))
	}
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

	eval := &stubEvaluator{
		id:            "opa",
		fileExtension: ".rego",
		requiredFiles: []string{"complytime-mapping.json"},
		testResults: &evaluator.TestResults{
			Total:  1,
			Passed: 1,
			Failed: 0,
		},
	}

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      eval,
		RunTests:       true,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(report.Requirements) != 1 {
		t.Fatalf("Requirements count = %d, want 1", len(report.Requirements))
	}
	if report.Requirements[0].Status != StatusImplementedPassing {
		t.Errorf("status = %q, want %q", report.Requirements[0].Status, StatusImplementedPassing)
	}
	if report.Metrics.Passing != 1 {
		t.Errorf("Passing = %d, want 1", report.Metrics.Passing)
	}
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

	eval := &stubEvaluator{
		id:            "opa",
		fileExtension: ".rego",
		requiredFiles: []string{"complytime-mapping.json"},
		testResults: &evaluator.TestResults{
			Total:  1,
			Passed: 0,
			Failed: 1,
			Errors: []string{"test_ctl_001_ar1 failed"},
		},
	}

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      eval,
		RunTests:       true,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(report.Requirements) != 1 {
		t.Fatalf("Requirements count = %d, want 1", len(report.Requirements))
	}
	if report.Requirements[0].Status != StatusImplementedFailing {
		t.Errorf("status = %q, want %q", report.Requirements[0].Status, StatusImplementedFailing)
	}
	if report.Metrics.Failing != 1 {
		t.Errorf("Failing = %d, want 1", report.Metrics.Failing)
	}
	if len(report.Requirements[0].TestErrors) != 1 {
		t.Errorf("TestErrors count = %d, want 1", len(report.Requirements[0].TestErrors))
	}
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

	eval := &stubEvaluator{
		id:            "opa",
		fileExtension: ".rego",
		requiredFiles: []string{"complytime-mapping.json"},
	}

	report, err := Run(context.Background(), Options{
		ResolvedPolicy: rp,
		PolicyDir:      dir,
		Evaluator:      eval,
		RunTests:       false,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if report.Requirements[0].Status != StatusImplemented {
		t.Errorf("status = %q, want %q (no test enrichment)", report.Requirements[0].Status, StatusImplemented)
	}
	if report.Metrics.Passing != 0 {
		t.Errorf("Passing = %d, want 0", report.Metrics.Passing)
	}
	if report.Metrics.Failing != 0 {
		t.Errorf("Failing = %d, want 0", report.Metrics.Failing)
	}
}

func TestRun_MissingRequiredInputs(t *testing.T) {
	eval := &stubEvaluator{
		id:            "opa",
		fileExtension: ".rego",
		requiredFiles: []string{"complytime-mapping.json"},
	}

	tests := []struct {
		name string
		opts Options
	}{
		{
			name: "nil resolved policy",
			opts: Options{PolicyDir: "/tmp", Evaluator: eval},
		},
		{
			name: "empty policy dir",
			opts: Options{ResolvedPolicy: &requirement.ResolvedPolicy{}, Evaluator: eval},
		},
		{
			name: "nil evaluator",
			opts: Options{ResolvedPolicy: &requirement.ResolvedPolicy{}, PolicyDir: "/tmp"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Run(context.Background(), tc.opts)
			if err == nil {
				t.Error("expected error for missing input, got nil")
			}
		})
	}
}

func TestComputeMetrics(t *testing.T) {
	entries := []RequirementEntry{
		{RequirementID: "R1", Status: StatusImplementedPassing},
		{RequirementID: "R2", Status: StatusImplementedFailing},
		{RequirementID: "R3", Status: StatusImplemented},
		{RequirementID: "R4", Status: StatusGap},
		{RequirementID: "R5", Status: StatusGap},
	}

	m := computeMetrics(entries)

	if m.TotalAutomated != 5 {
		t.Errorf("TotalAutomated = %d, want 5", m.TotalAutomated)
	}
	if m.Implemented != 3 {
		t.Errorf("Implemented = %d, want 3", m.Implemented)
	}
	if m.Gaps != 2 {
		t.Errorf("Gaps = %d, want 2", m.Gaps)
	}
	if m.Passing != 1 {
		t.Errorf("Passing = %d, want 1", m.Passing)
	}
	if m.Failing != 1 {
		t.Errorf("Failing = %d, want 1", m.Failing)
	}
	if m.CoveragePercent != 60 {
		t.Errorf("CoveragePercent = %f, want 60", m.CoveragePercent)
	}
}
