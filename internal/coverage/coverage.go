// SPDX-License-Identifier: Apache-2.0

// Package coverage compares a Policy's in-scope assessment requirements
// against enforcement artifacts in a policy directory, producing a
// structured coverage report. It also provides test-to-requirement mapping
// and attribution, classifying each requirement's test status based on
// per-test results.
package coverage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/internal/requirement"
	"github.com/complytime/complypack/internal/testresult"
)

// ---------------------------------------------------------------------------
// Gap-reporting coverage engine
// ---------------------------------------------------------------------------

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
	Version  string         `json:"version"`
	Mappings []MappingEntry `json:"mappings"`
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

	// Validate policy directory exists and is a directory
	info, err := os.Stat(opts.PolicyDir)
	if err != nil {
		return nil, fmt.Errorf("policy directory %q: %w", opts.PolicyDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("policy directory %q is not a directory", opts.PolicyDir)
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

	// Note: *.rego already matches *_test.rego, so a single glob covers both
	for _, path := range matches {
		if err := ctx.Err(); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading policy file %s: %w", path, err)
		}
		files[filepath.Base(path)] = string(data)
	}

	// Also read the mapping file if present (skip if not found, fail on other errors)
	requiredFiles := opts.Evaluator.RequiredFiles()
	for _, rf := range requiredFiles {
		rfPath := filepath.Join(opts.PolicyDir, rf)
		data, err := os.ReadFile(rfPath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("reading required file %s: %w", rf, err)
			}
			continue
		}
		files[rf] = string(data)
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
		// Gap-reporting uses aggregate test results (not per-requirement).
		// Per-requirement attribution is available via AttributeTests() and
		// is surfaced in the CLI and MCP transport layers. Within gap
		// reporting, mark all implemented requirements as failing when any
		// test fails to maintain conservative status reporting.
		report.Warnings = append(report.Warnings, Warning{
			Message: fmt.Sprintf(
				"test results are aggregate: %d of %d tests failed — "+
					"all implemented requirements are marked as failing; "+
					"use per-requirement attribution for granular status",
				results.Failed, results.Total),
		})
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

// ---------------------------------------------------------------------------
// Per-requirement test attribution engine
// ---------------------------------------------------------------------------

// RequirementTestStatus represents the test status of a requirement.
type RequirementTestStatus string

const (
	// Passing indicates all tests for the requirement passed.
	Passing RequirementTestStatus = "passing"
	// Failing indicates at least one test for the requirement failed.
	Failing RequirementTestStatus = "failing"
	// Untested indicates no tests were found for the requirement.
	Untested RequirementTestStatus = "untested"
)

// PackageMapping maps Rego package paths to requirement IDs.
// Each key is a full Rego package path (e.g., "data.policy.ac_2_1_test")
// and each value is a list of requirement IDs that package covers.
type PackageMapping map[string][]string

// requirementPattern matches package segments that follow the
// {family}_{major}[_{minor}...]_test naming convention.
// Family is lowercase alphabetic (e.g., "ac", "sc", "cp"), followed by
// underscore-separated numeric/alpha segments. Uppercase families (AC_2_1)
// and hyphenated families (cp-abc_1_2) are not matched by convention;
// use a coverage-mapping.yaml override file for non-standard naming.
var requirementPattern = regexp.MustCompile(`^([a-z]+)_(\d+(?:_[\da-z]+)*)$`)

// NormalizePackageToRequirement normalizes a Rego package path segment
// into a requirement ID using the naming convention described in the design:
//  1. Take the last segment of the package path.
//  2. Strip the _test suffix if present.
//  3. Match against {family}_{major}[_{minor}...] pattern.
//  4. Convert family-to-major separator to hyphen.
//  5. Convert remaining underscores between segments to dots.
//  6. Return empty string if no match.
func NormalizePackageToRequirement(packagePath string) string {
	// Take the last segment of the package path.
	parts := strings.Split(packagePath, ".")
	segment := parts[len(parts)-1]

	// Strip _test suffix if present.
	segment = strings.TrimSuffix(segment, "_test")

	// Match against the requirement ID pattern.
	matches := requirementPattern.FindStringSubmatch(segment)
	if matches == nil {
		return ""
	}

	family := matches[1]    // e.g., "ac"
	remainder := matches[2] // e.g., "2_1" or "7" or "5_1_a"

	// Convert underscores in the remainder to dots.
	version := strings.ReplaceAll(remainder, "_", ".")

	// family-major.minor... with hyphen separator.
	return family + "-" + version
}

// BuildConventionMapping builds a PackageMapping from the test details
// using the naming convention. Only packages that match the convention
// pattern are included.
func BuildConventionMapping(details []testresult.Detail) PackageMapping {
	mapping := make(PackageMapping)
	seen := make(map[string]bool)

	for _, d := range details {
		if seen[d.Package] {
			continue
		}
		seen[d.Package] = true

		reqID := NormalizePackageToRequirement(d.Package)
		if reqID == "" {
			continue
		}
		mapping[d.Package] = []string{reqID}
	}

	return mapping
}

// overrideFile represents the YAML structure of the coverage-mapping.yaml file.
type overrideFile struct {
	Version  string              `yaml:"version"`
	Mappings map[string][]string `yaml:"mappings"`
}

// LoadOverrideMapping loads a coverage-mapping.yaml override file.
// Returns nil mapping and nil error if the file does not exist.
// Returns a parse error if the file exists but is malformed.
// Returns an empty mapping (not nil) if the file has empty mappings.
func LoadOverrideMapping(filePath string) (PackageMapping, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading override mapping file: %w", err)
	}

	var of overrideFile
	if err := yaml.Unmarshal(data, &of); err != nil {
		return nil, fmt.Errorf("parsing override mapping file: %w", err)
	}

	if of.Version != "" && of.Version != "1" {
		return nil, fmt.Errorf("unsupported coverage mapping version %q; this version of complypack supports version \"1\" — upgrade complypack or use version \"1\"", of.Version)
	}

	if len(of.Mappings) == 0 {
		return make(PackageMapping), nil
	}

	mapping := make(PackageMapping, len(of.Mappings))
	for pkg, reqIDs := range of.Mappings {
		mapping[pkg] = reqIDs
	}
	return mapping, nil
}

// EnrichWithTestResults classifies each requirement's test status based
// on per-test details and the provided package-to-requirement mapping.
//
// For each requirement in the mapping:
//   - If no test details exist for any mapped package -> Untested
//   - If any test in any mapped package fails -> Failing
//   - If all tests in all mapped packages pass -> Passing
//
// If mapping or results is nil, returns an empty map.
// If results.Details is empty/nil, all mapped requirements are Untested.
func EnrichWithTestResults(mapping PackageMapping, results *evaluator.TestResults) map[string]RequirementTestStatus {
	if mapping == nil || results == nil {
		return make(map[string]RequirementTestStatus)
	}

	// Build reverse index: requirement ID -> list of package paths.
	reqPackages := make(map[string][]string)
	for pkg, reqIDs := range mapping {
		for _, reqID := range reqIDs {
			reqPackages[reqID] = append(reqPackages[reqID], pkg)
		}
	}

	// Index test details by package path.
	type pkgStatus struct {
		hasTests bool
		hasFail  bool
	}
	pkgStatuses := make(map[string]*pkgStatus)
	for _, d := range results.Details {
		ps, ok := pkgStatuses[d.Package]
		if !ok {
			ps = &pkgStatus{}
			pkgStatuses[d.Package] = ps
		}
		ps.hasTests = true
		if !d.Passed {
			ps.hasFail = true
		}
	}

	// Classify each requirement.
	result := make(map[string]RequirementTestStatus, len(reqPackages))
	for reqID, packages := range reqPackages {
		hasAnyTest := false
		hasFail := false

		for _, pkg := range packages {
			ps, ok := pkgStatuses[pkg]
			if !ok {
				continue
			}
			if ps.hasTests {
				hasAnyTest = true
			}
			if ps.hasFail {
				hasFail = true
			}
		}

		switch {
		case !hasAnyTest:
			result[reqID] = Untested
		case hasFail:
			result[reqID] = Failing
		default:
			result[reqID] = Passing
		}
	}

	return result
}

// AttributeTests is the high-level orchestration function that builds
// a PackageMapping (convention + optional override file) and returns
// per-requirement test status. This is the single entry point called
// by transport layers.
//
// contentDir is the directory containing the policy content (used to
// discover the override mapping file). mappingPath overrides the default
// file location. results contains the test execution results with
// per-test details.
//
// Returns an empty map (not nil) if results is nil or has no Details.
func AttributeTests(contentDir string, mappingPath string, results *evaluator.TestResults) (map[string]RequirementTestStatus, error) {
	if results == nil || len(results.Details) == 0 {
		return make(map[string]RequirementTestStatus), nil
	}

	// Build convention-based mapping from test details.
	mapping := BuildConventionMapping(results.Details)

	// Load optional override file.
	overridePath := mappingPath
	if overridePath == "" && contentDir != "" {
		overridePath = filepath.Join(contentDir, "coverage-mapping.yaml")
	}

	if overridePath != "" {
		overrides, err := LoadOverrideMapping(overridePath)
		if err != nil {
			return nil, fmt.Errorf("loading coverage mapping: %w", err)
		}
		// Merge overrides into mapping (overrides take precedence).
		for pkg, reqIDs := range overrides {
			mapping[pkg] = reqIDs
		}
	}

	// Enrich with test results.
	return EnrichWithTestResults(mapping, results), nil
}
