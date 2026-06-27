// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOPA_ID(t *testing.T) {
	opa := &OPA{}
	assert.Equal(t, "opa", opa.ID())
}

func TestOPA_Validate(t *testing.T) {
	opa := &OPA{}

	t.Run("valid Rego", func(t *testing.T) {
		src := `package example
import rego.v1

allow if {
	input.user == "admin"
}`
		errs := opa.Validate("test.rego", src)
		assert.Empty(t, errs)
	})

	t.Run("invalid Rego", func(t *testing.T) {
		src := `package example
allow {  # Missing import rego.v1
	input.user ==
}`
		errs := opa.Validate("test.rego", src)
		assert.NotEmpty(t, errs)
	})
}

func TestOPA_CheckContract(t *testing.T) {
	opa := &OPA{}

	ctx := cuecontext.New()
	val := ctx.CompileString(`
#Root: {
	apiVersion: string
	kind: string
	metadata?: {
		name?: string
	}
}
`)
	require.NoError(t, val.Err())
	schema := val.LookupPath(cue.MakePath(cue.Def("Root")))
	require.True(t, schema.Exists())

	t.Run("valid contract", func(t *testing.T) {
		src := `package example
import rego.v1

deny contains msg if {
	input.kind
	input.metadata.name
	msg := "test"
}`
		violations, err := opa.CheckContract("test.rego", src, schema)
		require.NoError(t, err)
		assert.Empty(t, violations)
	})

	t.Run("invalid contract", func(t *testing.T) {
		src := `package example
import rego.v1

deny contains msg if {
	input.nonexistent.field
	msg := "test"
}`
		violations, err := opa.CheckContract("test.rego", src, schema)
		require.NoError(t, err)
		assert.Len(t, violations, 1)
		assert.Contains(t, violations[0].Path, "nonexistent")
	})
}

func TestOPA_Test(t *testing.T) {
	opa := &OPA{}
	ctx := context.Background()

	t.Run("passing tests", func(t *testing.T) {
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

		results, err := opa.Test(ctx, files)
		require.NoError(t, err)
		assert.Equal(t, 1, results.Total)
		assert.Equal(t, 1, results.Passed)
		assert.Equal(t, 0, results.Failed)
	})

	t.Run("failing tests", func(t *testing.T) {
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

		results, err := opa.Test(ctx, files)
		require.NoError(t, err)
		assert.Equal(t, 1, results.Total)
		assert.Equal(t, 0, results.Passed)
		assert.Equal(t, 1, results.Failed)
		assert.NotEmpty(t, results.Errors)
	})
}

func TestOPA_Lint(t *testing.T) {
	opa := &OPA{}

	src := `package example
import rego.v1

allow if {
	input.user == "admin"
}`

	// Lint may return nil if regal is not installed (graceful degradation)
	warnings, err := opa.Lint("test.rego", src)
	assert.NoError(t, err)
	// Don't assert on warnings content since regal may or may not be installed
	t.Logf("Lint warnings: %v", warnings)
}

func TestOPA_FileExtension(t *testing.T) {
	opa := &OPA{}
	assert.Equal(t, ".rego", opa.FileExtension())
}

func TestDefaultRegistry(t *testing.T) {
	registry := DefaultRegistry()
	require.NotNil(t, registry)

	// Should have OPA registered
	opa, err := registry.Get(OPAEvaluatorID)
	require.NoError(t, err)
	assert.NotNil(t, opa)
	assert.Equal(t, OPAEvaluatorID, opa.ID())

	// Should be in IDs list
	ids := registry.IDs()
	assert.Contains(t, ids, OPAEvaluatorID)
}
