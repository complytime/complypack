// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testingMock extends mockEvaluator with configurable Test() return values.
type testingMock struct {
	mockEvaluator
	testResults *TestResults
	testErr     error
}

func (m *testingMock) Test(
	_ context.Context, _ map[string]string,
) (*TestResults, error) {
	return m.testResults, m.testErr
}

func TestTestPolicy_AllPass(t *testing.T) {
	eval := &testingMock{
		mockEvaluator: mockEvaluator{id: "test"},
		testResults: &TestResults{
			Total:  3,
			Passed: 3,
			Failed: 0,
			Errors: nil,
		},
	}

	result, err := TestPolicy(context.Background(), eval, "package p", nil)

	require.NoError(t, err)
	assert.True(t, result.TestDataValid)
	assert.Empty(t, result.TestDataErrors)
	assert.True(t, result.TestsExecuted)
	require.NotNil(t, result.Results)
	assert.Equal(t, 3, result.Results.Total)
	assert.Equal(t, 3, result.Results.Passed)
	assert.Equal(t, 0, result.Results.Failed)
	// nil Errors should be normalized to empty slice
	assert.NotNil(t, result.Results.Errors)
	assert.Empty(t, result.Results.Errors)
}

func TestTestPolicy_TestDataErrors(t *testing.T) {
	eval := &testingMock{
		mockEvaluator: mockEvaluator{id: "test"},
		testResults: &TestResults{
			Total: 5, Passed: 5,
		},
	}

	dataErrors := []string{
		"field 'metadata' is required",
		"field 'spec.replicas' must be integer",
	}

	result, err := TestPolicy(context.Background(), eval, "package p", dataErrors)

	require.NoError(t, err)
	assert.False(t, result.TestDataValid)
	require.Len(t, result.TestDataErrors, 2)
	assert.Equal(t, dataErrors, result.TestDataErrors)
	// Tests must NOT be executed when test data is invalid
	assert.False(t, result.TestsExecuted)
	assert.Nil(t, result.Results)
}

func TestTestPolicy_EmptyTestDataErrors(t *testing.T) {
	eval := &testingMock{
		mockEvaluator: mockEvaluator{id: "test"},
		testResults: &TestResults{
			Total: 1, Passed: 1,
		},
	}

	// Empty slice (not nil) should NOT trigger short-circuit
	result, err := TestPolicy(context.Background(), eval, "package p", []string{})

	require.NoError(t, err)
	assert.True(t, result.TestDataValid)
	assert.True(t, result.TestsExecuted)
	require.NotNil(t, result.Results)
}

func TestTestPolicy_EvaluatorError(t *testing.T) {
	eval := &testingMock{
		mockEvaluator: mockEvaluator{id: "test"},
		testErr:       errors.New("evaluator crashed"),
	}

	result, err := TestPolicy(context.Background(), eval, "package p", nil)

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "evaluator crashed")
}

func TestTestPolicy_FailedTests(t *testing.T) {
	eval := &testingMock{
		mockEvaluator: mockEvaluator{id: "test"},
		testResults: &TestResults{
			Total:  4,
			Passed: 2,
			Failed: 2,
			Errors: []string{"test_deny_no_labels: FAIL", "test_deny_missing_name: FAIL"},
		},
	}

	result, err := TestPolicy(context.Background(), eval, "package p", nil)

	require.NoError(t, err)
	assert.True(t, result.TestDataValid)
	assert.True(t, result.TestsExecuted)
	require.NotNil(t, result.Results)
	assert.Equal(t, 2, result.Results.Failed)
	assert.Len(t, result.Results.Errors, 2)
}

func TestTestPolicy_NilTestDataErrors(t *testing.T) {
	eval := &testingMock{
		mockEvaluator: mockEvaluator{id: "test"},
		testResults: &TestResults{
			Total: 1, Passed: 1,
		},
	}

	// nil testDataErrors should NOT trigger short-circuit
	result, err := TestPolicy(context.Background(), eval, "package p", nil)

	require.NoError(t, err)
	assert.True(t, result.TestDataValid)
	assert.True(t, result.TestsExecuted)
}
