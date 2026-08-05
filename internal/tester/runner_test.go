// SPDX-License-Identifier: Apache-2.0

package tester

import (
	"context"
	"testing"

	"github.com/complytime/complypack/internal/testresult"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		files      map[string]string
		wantTotal  int
		wantPassed int
		wantFailed int
		wantErr    bool
	}{
		{
			name: "all tests pass",
			files: map[string]string{
				"policy_test.rego": `package example
import rego.v1

test_allow if {
	allow with input as {"user": "admin"}
}

allow if {
	input.user == "admin"
}`,
			},
			wantTotal:  1,
			wantPassed: 1,
			wantFailed: 0,
			wantErr:    false,
		},
		{
			name: "one test fails",
			files: map[string]string{
				"policy_test.rego": `package example
import rego.v1

test_pass if {
	allow with input as {"user": "admin"}
}

test_fail if {
	# This test expects allow to be true for guest, but it won't be
	allow with input as {"user": "guest"}
}

allow if {
	input.user == "admin"
}`,
			},
			wantTotal:  2,
			wantPassed: 1,
			wantFailed: 1,
			wantErr:    false,
		},
		{
			name: "parse error",
			files: map[string]string{
				"bad.rego": `package example
allow {  # Missing import rego.v1
	input.user == "admin"
`,
			},
			wantErr: true,
		},
		{
			name:       "empty files map",
			files:      map[string]string{},
			wantTotal:  0,
			wantPassed: 0,
			wantFailed: 0,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := Run(ctx, tt.files)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantTotal, results.Total, "total tests")
			assert.Equal(t, tt.wantPassed, results.Passed, "passed tests")
			assert.Equal(t, tt.wantFailed, results.Failed, "failed tests")

			if tt.wantFailed > 0 {
				assert.NotEmpty(t, results.Errors, "should have error messages for failures")
			}
		})
	}
}

func TestRunDetails(t *testing.T) {
	ctx := context.Background()

	t.Run("all pass captures details", func(t *testing.T) {
		files := map[string]string{
			"policy_test.rego": `package policy.ac_2_1_test
import rego.v1

test_allow if {
	allow with input as {"user": "admin"}
}

allow if {
	input.user == "admin"
}`,
		}
		results, err := Run(ctx, files)
		require.NoError(t, err)
		assert.Equal(t, 1, results.Total)
		assert.Equal(t, 1, results.Passed)
		require.Len(t, results.Details, 1)
		assert.Equal(t, "test_allow", results.Details[0].Name)
		assert.Equal(t, "data.policy.ac_2_1_test", results.Details[0].Package)
		assert.True(t, results.Details[0].Passed)
		assert.Empty(t, results.Details[0].Error)
		assert.NotEmpty(t, results.Details[0].Location)
	})

	t.Run("mixed pass and fail", func(t *testing.T) {
		files := map[string]string{
			"policy_test.rego": `package policy.sc_7_test
import rego.v1

test_pass if {
	allow with input as {"user": "admin"}
}

test_fail if {
	allow with input as {"user": "guest"}
}

allow if {
	input.user == "admin"
}`,
		}
		results, err := Run(ctx, files)
		require.NoError(t, err)
		assert.Equal(t, 2, results.Total)
		assert.Equal(t, 1, results.Passed)
		assert.Equal(t, 1, results.Failed)

		// len(Details) == Passed + Failed (not Total)
		require.Len(t, results.Details, 2)

		// Find the failing detail
		var failDetail *testresult.Detail
		var passDetail *testresult.Detail
		for i := range results.Details {
			if !results.Details[i].Passed {
				failDetail = &results.Details[i]
			} else {
				passDetail = &results.Details[i]
			}
		}

		require.NotNil(t, failDetail, "should have a failing detail")
		assert.Equal(t, "data.policy.sc_7_test", failDetail.Package)
		assert.Equal(t, "test failed", failDetail.Error)
		assert.False(t, failDetail.Passed)

		require.NotNil(t, passDetail, "should have a passing detail")
		assert.True(t, passDetail.Passed)
		assert.Empty(t, passDetail.Error)
	})

	t.Run("skipped tests excluded from details", func(t *testing.T) {
		// OPA skipped tests: tests that complete without failing are passing,
		// tests that skip via an unsatisfied body don't fail — they're counted
		// differently by OPA's runner. We verify Details only includes
		// non-skipped tests (len(Details) == Passed + Failed).
		files := map[string]string{
			"policy_test.rego": `package example
import rego.v1

test_allow if {
	allow with input as {"user": "admin"}
}

allow if {
	input.user == "admin"
}`,
		}
		results, err := Run(ctx, files)
		require.NoError(t, err)
		assert.Len(t, results.Details, results.Passed+results.Failed,
			"len(Details) should equal Passed + Failed")
	})

	t.Run("package path extracted from AST", func(t *testing.T) {
		files := map[string]string{
			"policy_test.rego": `package policy.container_security_test
import rego.v1

test_allow if {
	allow with input as {"user": "admin"}
}

allow if {
	input.user == "admin"
}`,
		}
		results, err := Run(ctx, files)
		require.NoError(t, err)
		require.Len(t, results.Details, 1)
		assert.Equal(t, "data.policy.container_security_test", results.Details[0].Package)
	})

	t.Run("empty file map returns empty details", func(t *testing.T) {
		results, err := Run(ctx, map[string]string{})
		require.NoError(t, err)
		assert.NotNil(t, results.Details, "Details should not be nil")
		assert.Empty(t, results.Details, "Details should be empty")
		assert.Equal(t, 0, results.Total)
	})

	t.Run("test fails without error object", func(t *testing.T) {
		// Assertion failure (result.Fail: true, result.Error: nil)
		files := map[string]string{
			"policy_test.rego": `package example
import rego.v1

test_fail if {
	allow with input as {"user": "guest"}
}

allow if {
	input.user == "admin"
}`,
		}
		results, err := Run(ctx, files)
		require.NoError(t, err)
		require.Len(t, results.Details, 1)
		assert.False(t, results.Details[0].Passed)
		assert.Equal(t, "test failed", results.Details[0].Error)
	})
}
