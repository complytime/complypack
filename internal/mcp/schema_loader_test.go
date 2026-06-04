// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitModuleVersion(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantPath    string
		wantVersion string
	}{
		{
			name:        "no version",
			input:       "cue.dev/x/githubactions",
			wantPath:    "cue.dev/x/githubactions",
			wantVersion: "",
		},
		{
			name:        "explicit version",
			input:       "cue.dev/x/githubactions@v0.2.0",
			wantPath:    "cue.dev/x/githubactions",
			wantVersion: "v0.2.0",
		},
		{
			name:        "latest keyword",
			input:       "cue.dev/x/githubactions@latest",
			wantPath:    "cue.dev/x/githubactions",
			wantVersion: "latest",
		},
		{
			name:        "major version suffix only",
			input:       "cue.dev/x/githubactions@v0",
			wantPath:    "cue.dev/x/githubactions@v0",
			wantVersion: "",
		},
		{
			name:        "major version suffix with version",
			input:       "github.com/org/mod@v2@v2.1.0",
			wantPath:    "github.com/org/mod@v2",
			wantVersion: "v2.1.0",
		},
		{
			name:        "v0.latest shorthand",
			input:       "cue.dev/x/githubactions@v0.latest",
			wantPath:    "cue.dev/x/githubactions",
			wantVersion: "v0.latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotVersion := splitModuleVersion(tt.input)
			assert.Equal(t, tt.wantPath, gotPath)
			assert.Equal(t, tt.wantVersion, gotVersion)
		})
	}
}

func TestIsMajorOnly(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"v0", true},
		{"v1", true},
		{"v2", true},
		{"v12", true},
		{"v0.1.0", false},
		{"v0.latest", false},
		{"latest", false},
		{"v", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, isMajorOnly(tt.input))
		})
	}
}

func TestResolveCUEDefinition(t *testing.T) {
	ctx := cuecontext.New()

	t.Run("resolves named definition", func(t *testing.T) {
		val := ctx.CompileString(`
#Workflow: {
	name?: string
	on?: _
	jobs?: [string]: #Job
}
#Job: {
	"runs-on"?: string
	...
}
`)
		require.NoError(t, val.Err())

		resolved, err := resolveCUEDefinition(val, "Workflow")
		require.NoError(t, err)

		name := resolved.LookupPath(cue.MakePath(cue.Str("name").Optional()))
		assert.True(t, name.Exists(), "resolved value should have 'name' field")
	})

	t.Run("no fragment with regular fields passes through", func(t *testing.T) {
		val := ctx.CompileString(`
name?: string
on?: _
`)
		require.NoError(t, val.Err())

		resolved, err := resolveCUEDefinition(val, "")
		require.NoError(t, err)

		name := resolved.LookupPath(cue.MakePath(cue.Str("name").Optional()))
		assert.True(t, name.Exists(), "value should pass through unchanged")
	})

	t.Run("no fragment with definitions only returns error", func(t *testing.T) {
		val := ctx.CompileString(`
#Workflow: {
	name?: string
}
#Job: {
	stage?: string
}
`)
		require.NoError(t, val.Err())

		_, err := resolveCUEDefinition(val, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "#Workflow")
		assert.Contains(t, err.Error(), "#Job")
	})

	t.Run("nonexistent definition returns error", func(t *testing.T) {
		val := ctx.CompileString(`
#Workflow: {
	name?: string
}
`)
		require.NoError(t, val.Err())

		_, err := resolveCUEDefinition(val, "Bogus")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "#Bogus")
	})
}

func TestLoadFromCUERegistry_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	val, err := loadFromCUERegistry(ctx, "cue.dev/x/githubactions@v0")
	require.NoError(t, err, "loadFromCUERegistry should succeed for cue.dev/x/githubactions")

	// The loaded value should have a #Workflow definition
	workflow := val.LookupPath(cue.ParsePath("#Workflow"))
	assert.True(t, workflow.Exists(), "expected #Workflow definition in githubactions module")
}
