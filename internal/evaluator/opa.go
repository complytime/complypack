// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"
	"fmt"

	"cuelang.org/go/cue"
	"github.com/complytime/complypack/internal/tester"
	"github.com/complytime/complypack/internal/validator"
	"github.com/open-policy-agent/regal/pkg/linter"
	"github.com/open-policy-agent/regal/pkg/rules"
)

const OPAEvaluatorID = "opa"

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
	input, err := rules.InputFromText(filename, src)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input for linting: %w", err)
	}

	l := linter.NewLinter().WithInputModules(&input)

	report, err := l.Lint(context.Background())
	if err != nil {
		return nil, fmt.Errorf("linting failed: %w", err)
	}

	var warnings []LintWarning
	for _, v := range report.Violations {
		warnings = append(warnings, LintWarning{
			Rule:     v.Category,
			Message:  v.Title,
			Location: fmt.Sprintf("%s:%d:%d", filename, v.Location.Row, v.Location.Column),
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
