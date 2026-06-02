// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"cuelang.org/go/cue"
	"github.com/complytime/complypack/internal/tester"
	"github.com/complytime/complypack/internal/validator"
)

const OPAEvaluatorID = "io.complytime.opa"

// OPA implements the Evaluator interface for Open Policy Agent policies.
type OPA struct{}

func (o *OPA) ID() string {
	return OPAEvaluatorID
}

func (o *OPA) Validate(filename string, src string) []error {
	return validator.CheckRego(filename, src)
}

func (o *OPA) CheckContract(filename string, src string, schema cue.Value) ([]ContractViolation, error) {
	violations, err := validator.CheckContract(filename, src, schema)
	if err != nil {
		return nil, err
	}

	// Convert from validator.ContractViolation to evaluator.ContractViolation
	result := make([]ContractViolation, len(violations))
	for i, v := range violations {
		result[i] = ContractViolation{
			Path:     v.Path,
			Location: v.Location,
		}
	}

	return result, nil
}

func (o *OPA) Test(ctx context.Context, files map[string]string) (*TestResults, error) {
	results, err := tester.Run(ctx, files)
	if err != nil {
		return nil, err
	}

	// Convert from tester.Results to evaluator.TestResults
	return &TestResults{
		Total:  results.Total,
		Passed: results.Passed,
		Failed: results.Failed,
		Errors: results.Errors,
	}, nil
}

func (o *OPA) Lint(filename string, src string) ([]LintWarning, error) {
	// Check if regal is available
	if _, err := exec.LookPath("regal"); err != nil {
		// Regal not installed - graceful degradation
		return nil, nil
	}

	// Create temp file for policy
	tmpDir, err := os.MkdirTemp("", "opa-lint-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, filepath.Base(filename))
	if err := os.WriteFile(tmpFile, []byte(src), 0600); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	// Run regal lint
	cmd := exec.Command("regal", "lint", "--format", "json", tmpFile)
	output, err := cmd.CombinedOutput()
	// Intentionally ignore command error - regal returns non-zero when it finds linting issues,
	// which is expected behavior. We parse the JSON output regardless.
	_ = err

	// Parse JSON output
	var regalOutput struct {
		Violations []struct {
			Title    string `json:"title"`
			Category string `json:"category"`
			Location struct {
				File string `json:"file"`
				Row  int    `json:"row"`
				Col  int    `json:"col"`
			} `json:"location"`
		} `json:"violations"`
	}

	if err := json.Unmarshal(output, &regalOutput); err != nil {
		// If parsing fails, return no warnings (best-effort)
		return nil, nil
	}

	// Convert to LintWarning format
	var warnings []LintWarning
	for _, v := range regalOutput.Violations {
		warnings = append(warnings, LintWarning{
			Rule:     v.Category,
			Message:  v.Title,
			Location: fmt.Sprintf("%s:%d:%d", filename, v.Location.Row, v.Location.Col),
		})
	}

	return warnings, nil
}

func (o *OPA) FileExtension() string {
	return ".rego"
}

// DefaultRegistry creates a registry pre-populated with the OPA evaluator.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(&OPA{})
	return r
}
