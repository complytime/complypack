// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"errors"
	"testing"

	"cuelang.org/go/cue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validatingMock extends mockEvaluator with configurable return values
// for ValidatePolicy test scenarios.
type validatingMock struct {
	mockEvaluator
	validateErrors []error
	violations     []ContractViolation
	contractErr    error
	lintWarnings   []LintWarning
	lintErr        error
}

func (v *validatingMock) Validate(_ string, _ string) []error {
	return v.validateErrors
}

func (v *validatingMock) CheckContract(
	_ string, _ string, _ cue.Value,
) ([]ContractViolation, error) {
	return v.violations, v.contractErr
}

func (v *validatingMock) Lint(
	_ string, _ string,
) ([]LintWarning, error) {
	return v.lintWarnings, v.lintErr
}

func TestValidatePolicy_AllClear(t *testing.T) {
	eval := &validatingMock{
		mockEvaluator: mockEvaluator{id: "test"},
	}

	result := ValidatePolicy(eval, "package p", cue.Value{})

	assert.True(t, result.Valid)
	assert.Empty(t, result.SyntaxErrors)
	assert.Empty(t, result.ContractViolations)
	assert.Empty(t, result.LintWarnings)
	assert.NoError(t, result.LintErr)
}

func TestValidatePolicy_SyntaxErrors(t *testing.T) {
	eval := &validatingMock{
		mockEvaluator: mockEvaluator{id: "test"},
		validateErrors: []error{
			errors.New("unexpected token"),
			errors.New("missing closing brace"),
		},
	}

	result := ValidatePolicy(eval, "bad{", cue.Value{})

	assert.False(t, result.Valid)
	require.Len(t, result.SyntaxErrors, 2)
	assert.Equal(t, "unexpected token", result.SyntaxErrors[0])
	assert.Equal(t, "missing closing brace", result.SyntaxErrors[1])
	// Contract and lint are skipped when syntax errors exist
	assert.Empty(t, result.ContractViolations)
	assert.Empty(t, result.LintWarnings)
	assert.NoError(t, result.LintErr)
}

func TestValidatePolicy_ContractViolations(t *testing.T) {
	eval := &validatingMock{
		mockEvaluator: mockEvaluator{id: "test"},
		violations: []ContractViolation{
			{Path: "input.missing.field", Location: "policy.test:3:5"},
		},
	}

	result := ValidatePolicy(eval, "package p", cue.Value{})

	assert.False(t, result.Valid)
	assert.Empty(t, result.SyntaxErrors)
	require.Len(t, result.ContractViolations, 1)
	assert.Equal(t, "input.missing.field", result.ContractViolations[0].Path)
}

func TestValidatePolicy_ContractCheckInfraFailure(t *testing.T) {
	eval := &validatingMock{
		mockEvaluator: mockEvaluator{id: "test"},
		contractErr:   errors.New("CUE schema unavailable"),
	}

	result := ValidatePolicy(eval, "package p", cue.Value{})

	assert.False(t, result.Valid)
	require.Len(t, result.SyntaxErrors, 1)
	assert.Contains(t, result.SyntaxErrors[0], "contract check failed:")
	assert.Contains(t, result.SyntaxErrors[0], "CUE schema unavailable")
	// Violations should still be empty (infra error, not a violation)
	assert.Empty(t, result.ContractViolations)
}

func TestValidatePolicy_LintWarnings(t *testing.T) {
	eval := &validatingMock{
		mockEvaluator: mockEvaluator{id: "test"},
		lintWarnings: []LintWarning{
			{Rule: "no-unused-vars", Message: "x is unused", Location: "policy.test:5:1"},
		},
	}

	result := ValidatePolicy(eval, "package p", cue.Value{})

	// Lint warnings do not affect validity
	assert.True(t, result.Valid)
	assert.Empty(t, result.SyntaxErrors)
	assert.Empty(t, result.ContractViolations)
	require.Len(t, result.LintWarnings, 1)
	assert.Equal(t, "no-unused-vars", result.LintWarnings[0].Rule)
	assert.NoError(t, result.LintErr)
}

func TestValidatePolicy_LintError(t *testing.T) {
	lintErr := errors.New("linter binary not found")
	eval := &validatingMock{
		mockEvaluator: mockEvaluator{id: "test"},
		lintErr:       lintErr,
	}

	result := ValidatePolicy(eval, "package p", cue.Value{})

	// Lint errors are captured but do not affect validity
	assert.True(t, result.Valid)
	assert.Empty(t, result.SyntaxErrors)
	assert.Empty(t, result.ContractViolations)
	assert.ErrorIs(t, result.LintErr, lintErr)
}

func TestValidatePolicy_SyntaxErrorsSkipContractAndLint(t *testing.T) {
	contractCalled := false
	lintCalled := false

	eval := &trackingMock{
		validatingMock: validatingMock{
			mockEvaluator: mockEvaluator{id: "test"},
			validateErrors: []error{
				errors.New("parse error"),
			},
			violations: []ContractViolation{
				{Path: "should.not.appear", Location: "policy.test:1:1"},
			},
			lintWarnings: []LintWarning{
				{Rule: "should-not-appear", Message: "nope"},
			},
		},
		onContract: func() { contractCalled = true },
		onLint:     func() { lintCalled = true },
	}

	result := ValidatePolicy(eval, "bad", cue.Value{})

	assert.False(t, result.Valid)
	assert.False(t, contractCalled, "contract check must be skipped on syntax errors")
	assert.False(t, lintCalled, "lint must be skipped on syntax errors")
}

// trackingMock wraps validatingMock with callbacks to verify call patterns.
type trackingMock struct {
	validatingMock
	onContract func()
	onLint     func()
}

func (m *trackingMock) CheckContract(
	filename string, src string, schema cue.Value,
) ([]ContractViolation, error) {
	if m.onContract != nil {
		m.onContract()
	}
	return m.validatingMock.CheckContract(filename, src, schema)
}

func (m *trackingMock) Lint(
	filename string, src string,
) ([]LintWarning, error) {
	if m.onLint != nil {
		m.onLint()
	}
	return m.validatingMock.Lint(filename, src)
}

func TestValidatePolicy_FileExtensionUsed(t *testing.T) {
	var capturedFilename string
	eval := &filenameCapturer{
		validatingMock: validatingMock{
			mockEvaluator: mockEvaluator{id: "test"},
		},
		onValidate: func(f string) { capturedFilename = f },
	}

	ValidatePolicy(eval, "package p", cue.Value{})

	assert.Equal(t, "policy.mock", capturedFilename)
}

// filenameCapturer captures the filename passed to Validate.
type filenameCapturer struct {
	validatingMock
	onValidate func(filename string)
}

func (f *filenameCapturer) Validate(filename string, src string) []error {
	if f.onValidate != nil {
		f.onValidate(filename)
	}
	return f.validatingMock.Validate(filename, src)
}
