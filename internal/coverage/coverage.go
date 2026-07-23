// SPDX-License-Identifier: Apache-2.0

// Package coverage compares a Policy's in-scope assessment requirements
// against enforcement artifacts in a policy directory, producing a
// structured coverage report.
package coverage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/internal/requirement"
)

// RequirementStatus classifies the implementation state of an assessment requirement.
type RequirementStatus string

const (
	// StatusImplemented indicates an enforcement artifact exists but tests were not run.
	StatusImplemented RequirementStatus = "implemented"
	// StatusImplementedPassing indicates an enforcement artifact exists and tests pass.
	StatusImplementedPassing RequirementStatus = "implemented_passing"
	// StatusImplementedFailing indicates an enforcement artifact exists but tests fail.
	StatusImplementedFailing RequirementStatus = "implemented_failing"
	// StatusGap indicates no enforcement artifact exists for this requirement.
	StatusGap RequirementStatus = "gap"
)

// Options configures a coverage report run.
type Options struct {
	// ResolvedPolicy is the fully resolved policy to check coverage for.
	ResolvedPolicy *requirement.ResolvedPolicy

	// PolicyDir is the path to the directory containing enforcement artifacts.
	PolicyDir string

	// Evaluator is the policy-language evaluator to use for detection.
	Evaluator evaluator.Evaluator

	// RunTests enables test execution for pass/fail enrichment.
	RunTests bool
}

// Report is the structured output of a coverage run.
type Report struct {
	PolicyID     string             `json:"policy_id"`
	Requirements []RequirementEntry `json:"requirements"`
	Metrics      Metrics            `json:"metrics"`
	Warnings     []Warning          `json:"warnings"`
	Manual       []ManualEntry      `json:"manual,omitempty"`
}

// RequirementEntry describes the coverage status of a single assessment requirement.
type RequirementEntry struct {
	RequirementID string            `json:"requirement_id"`
	ControlID     string            `json:"control_id,omitempty"`
	Status        RequirementStatus `json:"status"`
	RegoPackage   string            `json:"rego_package,omitempty"`
	TestErrors    []string          `json:"test_errors,omitempty"`
}

// ManualEntry records a manual assessment requirement excluded from coverage metrics.
type ManualEntry struct {
	RequirementID string `json:"requirement_id"`
	PlanID        string `json:"plan_id"`
}

// Metrics contains aggregate coverage statistics.
type Metrics struct {
	TotalAutomated  int     `json:"total_automated"`
	Implemented     int     `json:"implemented"`
	Gaps            int     `json:"gaps"`
	CoveragePercent float64 `json:"coverage_percent"`
	Passing         int     `json:"passing,omitempty"`
	Failing         int     `json:"failing,omitempty"`
}

// Warning records a non-fatal issue encountered during the coverage scan.
type Warning struct {
	Message string `json:"message"`
}

// MappingFile represents the complytime-mapping.json structure.
type MappingFile struct {
	Version  string           `json:"version"`
	Mappings []MappingEntry   `json:"mappings"`
}

// MappingEntry maps a Rego package namespace to an assessment requirement ID.
type MappingEntry struct {
	ID            string `json:"id"`
	RequirementID string `json:"requirement_id"`
}

// parseMappingFile reads and parses a complytime-mapping.json file.
func parseMappingFile(path string) (*MappingFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading mapping file: %w", err)
	}

	var mf MappingFile
	if err := json.Unmarshal(data, &mf); err != nil {
		return nil, fmt.Errorf("parsing mapping file: %w", err)
	}

	return &mf, nil
}

// detectFromMappingFile locates and parses the evaluator's mapping file,
// returning a map of requirement ID -> Rego package name.
func detectFromMappingFile(policyDir string, eval evaluator.Evaluator) (map[string]string, error) {
	requiredFiles := eval.RequiredFiles()
	if len(requiredFiles) == 0 {
		return nil, fmt.Errorf("evaluator %q has no required files", eval.ID())
	}

	mappingPath := filepath.Join(policyDir, requiredFiles[0])
	mf, err := parseMappingFile(mappingPath)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(mf.Mappings))
	for _, entry := range mf.Mappings {
		result[entry.RequirementID] = entry.ID
	}

	return result, nil
}

// detectFromFileExtension scans the policy directory for files matching
// the evaluator's file extension and returns the count.
func detectFromFileExtension(policyDir string, eval evaluator.Evaluator) (int, error) {
	ext := eval.FileExtension()
	pattern := filepath.Join(policyDir, "*"+ext)

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return 0, fmt.Errorf("scanning for %s files: %w", ext, err)
	}

	return len(matches), nil
}

// buildControlIndex creates a map from requirement ID to control ID
// using the resolved policy's control catalogs.
func buildControlIndex(rp *requirement.ResolvedPolicy) map[string]string {
	index := make(map[string]string)
	for _, controlID := range rp.ControlIDs() {
		for _, req := range rp.RequirementsForControl(controlID) {
			index[req.Id] = controlID
		}
	}
	return index
}

// Run executes a coverage analysis comparing the resolved policy's
// automated assessment requirements against enforcement artifacts in
// the policy directory.
func Run(ctx context.Context, opts Options) (*Report, error) {
	if opts.ResolvedPolicy == nil {
		return nil, fmt.Errorf("resolved policy is required")
	}
	if opts.PolicyDir == "" {
		return nil, fmt.Errorf("policy directory is required")
	}
	if opts.Evaluator == nil {
		return nil, fmt.Errorf("evaluator is required")
	}

	// Triage to partition automated vs manual requirements
	triage := requirement.TriageAssessmentPlans(opts.ResolvedPolicy)

	// Build control index for grouping
	controlIndex := buildControlIndex(opts.ResolvedPolicy)

	// Collect automated requirement IDs
	automatedReqIDs := make(map[string]bool, len(triage.Automated))
	for _, plan := range triage.Automated {
		automatedReqIDs[plan.RequirementID] = true
	}

	report := &Report{
		PolicyID:     opts.ResolvedPolicy.Policy.Metadata.Id,
		Requirements: []RequirementEntry{},
		Warnings:     []Warning{},
	}

	// Record manual requirements
	for _, plan := range triage.Manual {
		report.Manual = append(report.Manual, ManualEntry{
			RequirementID: plan.RequirementID,
			PlanID:        plan.PlanID,
		})
	}

	// Detect implemented requirements via mapping file
	implementedReqs, err := detectFromMappingFile(opts.PolicyDir, opts.Evaluator)
	usedFallback := false
	if err != nil {
		// Fallback to file-extension scanning
		usedFallback = true
		count, scanErr := detectFromFileExtension(opts.PolicyDir, opts.Evaluator)
		if scanErr != nil {
			return nil, fmt.Errorf("detection failed: mapping file: %w; fallback: %w", err, scanErr)
		}
		report.Warnings = append(report.Warnings, Warning{
			Message: fmt.Sprintf(
				"mapping file not found, fell back to file-extension scanning; "+
					"detected %d %s file(s) but cannot determine requirement mapping — "+
					"all automated requirements reported as gaps",
				count, opts.Evaluator.FileExtension(),
			),
		})
		if count > 0 {
			report.Warnings = append(report.Warnings, Warning{
				Message: "file-extension fallback has reduced detection precision; " +
					"add a complytime-mapping.json for accurate coverage",
			})
		}
		implementedReqs = make(map[string]string)
	}

	// Classify each automated requirement
	for _, plan := range triage.Automated {
		entry := RequirementEntry{
			RequirementID: plan.RequirementID,
			ControlID:     controlIndex[plan.RequirementID],
		}

		if regoPackage, ok := implementedReqs[plan.RequirementID]; ok {
			entry.Status = StatusImplemented
			entry.RegoPackage = regoPackage
		} else {
			entry.Status = StatusGap
		}

		report.Requirements = append(report.Requirements, entry)
	}

	// Optionally enrich with test results
	if opts.RunTests && !usedFallback {
		if err := enrichWithTestResults(ctx, report, opts); err != nil {
			report.Warnings = append(report.Warnings, Warning{
				Message: fmt.Sprintf("test execution failed: %v", err),
			})
		}
	}

	// Compute metrics
	report.Metrics = computeMetrics(report.Requirements)

	return report, nil
}

// enrichWithTestResults runs the evaluator's Test() method and updates
// implemented requirements to passing or failing.
func enrichWithTestResults(ctx context.Context, report *Report, opts Options) error {
	// Read all policy files from the directory
	files := make(map[string]string)
	ext := opts.Evaluator.FileExtension()
	pattern := filepath.Join(opts.PolicyDir, "*"+ext)

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("scanning for test files: %w", err)
	}

	// Also include test files (e.g., *_test.rego)
	testPattern := filepath.Join(opts.PolicyDir, "*_test"+ext)
	testMatches, err := filepath.Glob(testPattern)
	if err != nil {
		return fmt.Errorf("scanning for test files: %w", err)
	}

	// Merge, dedup
	allFiles := make(map[string]bool)
	for _, m := range matches {
		allFiles[m] = true
	}
	for _, m := range testMatches {
		allFiles[m] = true
	}

	for path := range allFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading policy file %s: %w", path, err)
		}
		files[filepath.Base(path)] = string(data)
	}

	// Also read the mapping file if present
	requiredFiles := opts.Evaluator.RequiredFiles()
	for _, rf := range requiredFiles {
		rfPath := filepath.Join(opts.PolicyDir, rf)
		data, err := os.ReadFile(rfPath)
		if err == nil {
			files[rf] = string(data)
		}
	}

	if len(files) == 0 {
		return nil
	}

	results, err := opts.Evaluator.Test(ctx, files)
	if err != nil {
		return fmt.Errorf("running tests: %w", err)
	}

	// If all tests pass, mark all implemented as passing
	if results.Failed == 0 {
		for i := range report.Requirements {
			if report.Requirements[i].Status == StatusImplemented {
				report.Requirements[i].Status = StatusImplementedPassing
			}
		}
	} else {
		// Mark all implemented as failing (test results are aggregate, not per-requirement)
		for i := range report.Requirements {
			if report.Requirements[i].Status == StatusImplemented {
				report.Requirements[i].Status = StatusImplementedFailing
				report.Requirements[i].TestErrors = results.Errors
			}
		}
	}

	return nil
}

// computeMetrics calculates aggregate coverage statistics from requirement entries.
func computeMetrics(entries []RequirementEntry) Metrics {
	m := Metrics{
		TotalAutomated: len(entries),
	}

	for _, e := range entries {
		switch e.Status {
		case StatusImplemented, StatusImplementedPassing, StatusImplementedFailing:
			m.Implemented++
		case StatusGap:
			m.Gaps++
		}
		if e.Status == StatusImplementedPassing {
			m.Passing++
		}
		if e.Status == StatusImplementedFailing {
			m.Failing++
		}
	}

	if m.TotalAutomated > 0 {
		m.CoveragePercent = float64(m.Implemented) / float64(m.TotalAutomated) * 100
	}

	return m
}
