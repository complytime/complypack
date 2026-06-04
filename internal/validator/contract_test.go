// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/complytime/complypack/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckContract(t *testing.T) {
	// Load kubernetes CUE schema for testing
	cueSrc, err := schemas.GetBuiltInCUESchema("kubernetes")
	require.NoError(t, err)

	ctx := cuecontext.New()
	compiled := ctx.CompileBytes(cueSrc)
	require.NoError(t, compiled.Err())

	// Use the compiled schema directly - it contains the field definitions
	schema := compiled

	// Load CI CUE schema for testing top type and pattern constraints
	ciSrc, err := schemas.GetBuiltInCUESchema("ci")
	require.NoError(t, err)
	ciCompiled := ctx.CompileBytes(ciSrc)
	require.NoError(t, ciCompiled.Err())
	ciSchema := ciCompiled

	tests := []struct {
		name           string
		schema         cue.Value
		src            string
		wantViolations int
	}{
		{
			name:   "valid contract - references exist",
			schema: schema,
			src: `package example
import rego.v1

deny contains msg if {
	input.apiVersion
	input.kind
	input.metadata.name
	msg := "test"
}`,
			wantViolations: 0,
		},
		{
			name:   "missing path flagged",
			schema: schema,
			src: `package example
import rego.v1

deny contains msg if {
	input.nonexistent.field
	msg := "test"
}`,
			wantViolations: 1,
		},
		{
			name:   "dynamic refs skipped",
			schema: schema,
			src: `package example
import rego.v1

deny contains msg if {
	key := "name"
	input.metadata[key]
	msg := "test"
}`,
			wantViolations: 0,
		},
		{
			name:   "multiple violations",
			schema: schema,
			src: `package example
import rego.v1

deny contains msg if {
	input.nonexistent1
	input.nonexistent2.nested
	msg := "test"
}`,
			wantViolations: 2,
		},
		{
			name:   "input reference is valid",
			schema: schema,
			src: `package example
import rego.v1

deny contains msg if {
	input
	msg := "test"
}`,
			wantViolations: 0,
		},
		{
			name:   "CI top type - on.push.branches is valid",
			schema: ciSchema,
			src: `package example
import rego.v1

deny contains msg if {
	input.on.push.branches
	msg := "test"
}`,
			wantViolations: 0,
		},
		{
			name:   "CI pattern constraint - jobs.build is valid",
			schema: ciSchema,
			src: `package example
import rego.v1

deny contains msg if {
	input.jobs.build
	msg := "test"
}`,
			wantViolations: 0,
		},
		{
			name:   "CI pattern + nested field - jobs.build.steps is valid",
			schema: ciSchema,
			src: `package example
import rego.v1

deny contains msg if {
	job := input.jobs.build
	job.steps
	msg := "test"
}`,
			wantViolations: 0,
		},
		{
			name:   "K8s pattern constraint - metadata.labels.app is valid",
			schema: schema,
			src: `package example
import rego.v1

deny contains msg if {
	input.metadata.labels.app
	msg := "test"
}`,
			wantViolations: 0,
		},
		{
			name:   "CI open schema - arbitrary top-level key is valid",
			schema: ciSchema,
			src: `package example
import rego.v1

deny contains msg if {
	input.completely_bogus
	msg := "test"
}`,
			// CI schema has [string]: #Job | [...string] | _ which accepts any key
			wantViolations: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations, err := CheckContract("test.rego", tt.src, tt.schema)
			require.NoError(t, err)
			assert.Len(t, violations, tt.wantViolations)

			// Check that violations contain path and location
			for _, v := range violations {
				assert.NotEmpty(t, v.Path, "violation should have path")
				assert.NotEmpty(t, v.Location, "violation should have location")
				assert.Contains(t, v.Error(), v.Path, "Error() should include path")
				assert.Contains(t, v.Error(), v.Location, "Error() should include location")
			}
		})
	}
}

func TestCheckContractInvalidRego(t *testing.T) {
	ctx := cuecontext.New()
	schema := ctx.CompileString(`package kubernetes
apiVersion: string
kind: string`)
	require.NoError(t, schema.Err())

	src := `package example
allow {  // Missing import rego.v1 and malformed
	input.apiVersion ==
}`

	_, err := CheckContract("test.rego", src, schema)
	assert.Error(t, err, "should return error for invalid Rego")
	assert.Contains(t, err.Error(), "failed to parse Rego")
}
