// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"context"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCUELoader_Match(t *testing.T) {
	l := &CUELoader{}
	assert.True(t, l.Match("cue://cue.dev/x/k8s"))
	assert.False(t, l.Match("https://example.com"))
	assert.False(t, l.Match("file:///path"))
	assert.False(t, l.Match(""))
}

func TestSplitModuleVersion(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantPath    string
		wantVersion string
	}{
		{"no version", "cue.dev/x/githubactions", "cue.dev/x/githubactions", ""},
		{"explicit version", "cue.dev/x/githubactions@v0.2.0", "cue.dev/x/githubactions", "v0.2.0"},
		{"latest keyword", "cue.dev/x/githubactions@latest", "cue.dev/x/githubactions", "latest"},
		{"major version suffix only", "cue.dev/x/githubactions@v0", "cue.dev/x/githubactions@v0", ""},
		{"major with version", "github.com/org/mod@v2@v2.1.0", "github.com/org/mod@v2", "v2.1.0"},
		{"v0.latest shorthand", "cue.dev/x/githubactions@v0.latest", "cue.dev/x/githubactions", "v0.latest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotVersion := SplitModuleVersion(tt.input)
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
			assert.Equal(t, tt.want, IsMajorOnly(tt.input))
		})
	}
}

func TestImportPathForModule(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"cue.dev/x/githubactions@v0", "cue.dev/x/githubactions"},
		{"cue.dev/x/githubactions", "cue.dev/x/githubactions"},
		{"github.com/org/mod@v2", "github.com/org/mod"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, ImportPathForModule(tt.input))
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
		assert.True(t, name.Exists())
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
		assert.True(t, name.Exists())
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

	val, err := loadFromCUERegistry(ctx, "cue.dev/x/githubactions@v0", "")
	require.NoError(t, err)

	workflow := val.LookupPath(cue.ParsePath("#Workflow"))
	assert.True(t, workflow.Exists())
}

func TestSplitModuleAndSubpackage(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantModule string
		wantSubPkg string
	}{
		{"no subpackage", "cue.dev/x/githubactions@v0", "cue.dev/x/githubactions@v0", ""},
		{"with subpackage", "cue.dev/x/k8s.io@v0/api/apps/v1", "cue.dev/x/k8s.io@v0", "api/apps/v1"},
		{"deep subpackage", "cue.dev/x/k8s.io@v0/api/core/v1", "cue.dev/x/k8s.io@v0", "api/core/v1"},
		{"no major version", "cue.dev/x/githubactions", "cue.dev/x/githubactions", ""},
		{"explicit version no subpackage", "cue.dev/x/githubactions@v0.2.0", "cue.dev/x/githubactions@v0.2.0", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotModule, gotSubPkg := splitModuleAndSubpackage(tt.input)
			assert.Equal(t, tt.wantModule, gotModule)
			assert.Equal(t, tt.wantSubPkg, gotSubPkg)
		})
	}
}
