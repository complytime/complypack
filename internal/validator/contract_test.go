// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func compileClosedSchema(t *testing.T, src string) cue.Value {
	t.Helper()
	ctx := cuecontext.New()
	val := ctx.CompileString(src)
	require.NoError(t, val.Err())
	root := val.LookupPath(cue.MakePath(cue.Def("Root")))
	require.True(t, root.Exists(), "schema must define #Root")
	return root
}

func TestCheckContract(t *testing.T) {
	k8sSchema := compileClosedSchema(t, `
#Root: {
	apiVersion: string
	kind: string
	metadata?: {
		name?: string
		labels?: [string]: string
		annotations?: [string]: string
	}
	spec?: _
}
`)

	ciSchema := compileClosedSchema(t, `
#Root: {
	name?: string
	on?:   _
	jobs?: [string]: #Job
}

#Job: {
	"runs-on"?: string
	steps?: [...#Step]
	...
}

#Step: {
	uses?: string
	run?:  string
	name?: string
	...
}
`)

	disjunctionSchema := compileClosedSchema(t, `
#Root: {a?: string} | {b?: string}
`)

	tests := []struct {
		name           string
		schema         cue.Value
		src            string
		wantViolations int
	}{
		{
			name:   "valid contract - references exist",
			schema: k8sSchema,
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
			schema: k8sSchema,
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
			schema: k8sSchema,
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
			schema: k8sSchema,
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
			schema: k8sSchema,
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
			schema: k8sSchema,
			src: `package example
import rego.v1

deny contains msg if {
	input.metadata.labels.app
	msg := "test"
}`,
			wantViolations: 0,
		},
		{
			name:   "closed schema rejects arbitrary top-level key",
			schema: ciSchema,
			src: `package example
import rego.v1

deny contains msg if {
	input.completely_bogus
	msg := "test"
}`,
			wantViolations: 1,
		},
		{
			name:   "disjunction - field in first branch is valid",
			schema: disjunctionSchema,
			src: `package example
import rego.v1

deny contains msg if {
	input.a
	msg := "test"
}`,
			wantViolations: 0,
		},
		{
			name:   "disjunction - field in second branch is valid",
			schema: disjunctionSchema,
			src: `package example
import rego.v1

deny contains msg if {
	input.b
	msg := "test"
}`,
			wantViolations: 0,
		},
		{
			name:   "disjunction - field in neither branch is rejected",
			schema: disjunctionSchema,
			src: `package example
import rego.v1

deny contains msg if {
	input.c
	msg := "test"
}`,
			wantViolations: 1,
		},
		{
			name:   "closed definition rejects bogus nested path",
			schema: k8sSchema,
			src: `package example
import rego.v1

deny contains msg if {
	input.metadata.bogus_field
	msg := "test"
}`,
			wantViolations: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations, err := CheckContract("test.rego", tt.src, tt.schema)
			require.NoError(t, err)
			assert.Len(t, violations, tt.wantViolations)

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
	schema := ctx.CompileString(`apiVersion: string
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

func FuzzPathExistsInSchema(f *testing.F) {
	schema := compileClosedSchema(&testing.T{}, `
#Root: {
	apiVersion: string
	kind: string
	metadata?: {
		name?: string
		namespace?: string
		labels?: [string]: string
	}
	spec?: _
}
`)

	f.Add("input.apiVersion")
	f.Add("input.metadata.name")
	f.Add("input.metadata.labels.app")
	f.Add("input.spec.replicas")
	f.Add("input.nonexistent")
	f.Add("input")
	f.Add("input.metadata.name.too.deep")
	f.Add("")
	f.Add("input....")
	f.Add("input.metadata..name")

	f.Fuzz(func(t *testing.T, path string) {
		_ = pathExistsInSchema(path, schema)
	})
}
