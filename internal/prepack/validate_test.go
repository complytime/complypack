// SPDX-License-Identifier: Apache-2.0

package prepack

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/complytime/complypack/internal/evaluator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadTestCUESchemaInline(t *testing.T) cue.Value {
	t.Helper()
	ctx := cuecontext.New()
	val := ctx.CompileString(`
#Root: {
	apiVersion?: string
	kind?:       string
	metadata?: {
		name?:        string
		namespace?:   string
		labels?:      [string]: string
		annotations?: [string]: string
	}
	spec?: {
		replicas?: int
		template?: {
			spec?: {
				containers?: [...{
					name?:  string
					image?: string
					...
				}]
				...
			}
		}
		containers?: [...{
			name?:  string
			image?: string
			...
		}]
		...
	}
}
`)
	require.NoError(t, val.Err())
	root := val.LookupPath(cue.MakePath(cue.Def("Root")))
	require.True(t, root.Exists())
	return root
}

func TestValidate_ValidPolicies(t *testing.T) {
	ctx := context.Background()
	eval := &evaluator.OPA{}
	s := loadTestCUESchemaInline(t)

	result, err := Validate(ctx, "testdata/valid", eval, []cue.Value{s}, ValidationOptions{})
	require.NoError(t, err)

	assert.Equal(t, 1, result.FilesChecked)
	assert.Empty(t, result.SyntaxErrors)
	assert.Empty(t, result.ContractViolations)
	assert.True(t, result.Valid())
}

func TestValidate_SyntaxError(t *testing.T) {
	ctx := context.Background()
	eval := &evaluator.OPA{}
	s := loadTestCUESchemaInline(t)

	result, err := Validate(ctx, "testdata/syntax-error", eval, []cue.Value{s}, ValidationOptions{})
	require.NoError(t, err)

	assert.Equal(t, 1, result.FilesChecked)
	assert.NotEmpty(t, result.SyntaxErrors)
	assert.Empty(t, result.ContractViolations, "should skip contract check on syntax failure")
	assert.False(t, result.Valid())
}

func TestValidate_ContractViolation(t *testing.T) {
	ctx := context.Background()
	eval := &evaluator.OPA{}
	s := loadTestCUESchemaInline(t)

	result, err := Validate(ctx, "testdata/contract-violation", eval, []cue.Value{s}, ValidationOptions{})
	require.NoError(t, err)

	assert.Equal(t, 1, result.FilesChecked)
	assert.Empty(t, result.SyntaxErrors)
	assert.NotEmpty(t, result.ContractViolations)
	assert.False(t, result.Valid())
}

func TestValidate_EmptyDirectory(t *testing.T) {
	ctx := context.Background()
	eval := &evaluator.OPA{}
	s := loadTestCUESchemaInline(t)

	result, err := Validate(ctx, "testdata/empty", eval, []cue.Value{s}, ValidationOptions{})
	require.NoError(t, err)

	assert.Equal(t, 0, result.FilesChecked)
	assert.True(t, result.Valid())
}

func TestValidate_SkipTests(t *testing.T) {
	ctx := context.Background()
	eval := &evaluator.OPA{}
	s := loadTestCUESchemaInline(t)

	result, err := Validate(ctx, "testdata/valid", eval, []cue.Value{s}, ValidationOptions{
		SkipTests: true,
	})
	require.NoError(t, err)

	assert.True(t, result.TestsSkipped)
	assert.Nil(t, result.TestResults)
	assert.True(t, result.Valid())
}

func TestValidate_NoSchema(t *testing.T) {
	ctx := context.Background()
	eval := &evaluator.OPA{}

	result, err := Validate(ctx, "testdata/valid", eval, nil, ValidationOptions{})
	require.NoError(t, err)

	assert.Equal(t, 1, result.FilesChecked)
	assert.Empty(t, result.SyntaxErrors)
	assert.Empty(t, result.ContractViolations, "contract check should be skipped without schemas")
	assert.True(t, result.Valid())
}

func TestValidate_NonexistentDirectory(t *testing.T) {
	ctx := context.Background()
	eval := &evaluator.OPA{}

	_, err := Validate(ctx, "testdata/nonexistent", eval, nil, ValidationOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "collecting policy files")
}

func TestValidate_MultiSchema(t *testing.T) {
	ctx := context.Background()
	eval := &evaluator.OPA{}
	schemas := multiPlatformSchemas(t)

	result, err := Validate(ctx, "testdata/multi-platform", eval, schemas, ValidationOptions{
		SkipTests: true,
	})
	require.NoError(t, err)

	assert.Equal(t, 2, result.FilesChecked)
	assert.Empty(t, result.SyntaxErrors)
	assert.Empty(t, result.ContractViolations, "each policy should pass against at least one schema")
	assert.True(t, result.Valid())
}

func compileClosedSchema(t *testing.T, src string) cue.Value {
	t.Helper()
	cueCtx := cuecontext.New()
	val := cueCtx.CompileString(src)
	require.NoError(t, val.Err())
	root := val.LookupPath(cue.MakePath(cue.Def("Root")))
	require.True(t, root.Exists(), "schema must define #Root")
	return root
}

func multiPlatformSchemas(t *testing.T) []cue.Value {
	t.Helper()
	k8s := compileClosedSchema(t, `
#Root: {
	apiVersion?: string
	kind?:       string
	metadata?: {
		name?:      string
		namespace?: string
	}
	spec?: {
		replicas?: int
		template?: _
	}
}
`)
	ci := compileClosedSchema(t, `
#Root: {
	name?: string
	on?:   _
	jobs?: [string]: #Job
}
#Job: {
	"runs-on"?: string
	steps?: [...]
	...
}
`)
	return []cue.Value{k8s, ci}
}

func TestValidate_MultiSchema_AllReject(t *testing.T) {
	ctx := context.Background()
	eval := &evaluator.OPA{}
	schemas := multiPlatformSchemas(t)

	result, err := Validate(ctx, "testdata/multi-platform-violation", eval, schemas, ValidationOptions{
		SkipTests: true,
	})
	require.NoError(t, err)

	assert.Equal(t, 1, result.FilesChecked)
	assert.NotEmpty(t, result.ContractViolations, "bogus paths should be rejected by all schemas")
	assert.False(t, result.Valid())
}

func TestValidate_MultiSchema_MixedFields(t *testing.T) {
	ctx := context.Background()
	eval := &evaluator.OPA{}
	schemas := multiPlatformSchemas(t)

	result, err := Validate(ctx, "testdata/mixed-fields", eval, schemas, ValidationOptions{
		SkipTests: true,
	})
	require.NoError(t, err)

	assert.Equal(t, 1, result.FilesChecked)
	assert.NotEmpty(t, result.ContractViolations, "policy mixing k8s and CI fields should fail against all schemas")
	assert.False(t, result.Valid())
}

func TestValidate_SingleSchemaInSlice(t *testing.T) {
	ctx := context.Background()
	eval := &evaluator.OPA{}
	s := loadTestCUESchemaInline(t)

	result, err := Validate(ctx, "testdata/valid", eval, []cue.Value{s}, ValidationOptions{
		SkipTests: true,
	})
	require.NoError(t, err)

	assert.Equal(t, 1, result.FilesChecked)
	assert.Empty(t, result.ContractViolations)
	assert.True(t, result.Valid())

	result, err = Validate(ctx, "testdata/contract-violation", eval, []cue.Value{s}, ValidationOptions{
		SkipTests: true,
	})
	require.NoError(t, err)

	assert.NotEmpty(t, result.ContractViolations, "single schema should still catch violations")
	assert.False(t, result.Valid())
}

// multiFileEvaluator is a mock that declares multiple required companion
// files, used to verify the Stage 0 loop reports only missing entries.
type multiFileEvaluator struct {
	evaluator.OPA
	requiredFiles []string
}

func (m *multiFileEvaluator) RequiredFiles() []string {
	return m.requiredFiles
}

func TestValidate_MultipleRequiredFiles_PartialPresence(t *testing.T) {
	ctx := context.Background()
	s := loadTestCUESchemaInline(t)

	// Build a temp content directory with one required file present
	// and one absent, plus a valid policy file.
	contentDir := t.TempDir()
	presentFile := "file-a.json"
	absentFile := "file-b.json"

	err := os.WriteFile(
		filepath.Join(contentDir, presentFile),
		[]byte(`{}`), 0o600,
	)
	require.NoError(t, err)

	err = os.WriteFile(
		filepath.Join(contentDir, "policy.rego"),
		[]byte("package main\nimport rego.v1\n\ndeny contains msg if {\n"+
			"\tinput.kind == \"Pod\"\n\tmsg := \"test\"\n}\n"),
		0o600,
	)
	require.NoError(t, err)

	eval := &multiFileEvaluator{
		requiredFiles: []string{presentFile, absentFile},
	}

	result, err := Validate(
		ctx, contentDir, eval, []cue.Value{s}, ValidationOptions{},
	)
	require.NoError(t, err)

	assert.Equal(
		t, []string{absentFile}, result.MissingFiles,
		"only the absent file should appear in MissingFiles",
	)
	assert.Equal(
		t, 0, result.FilesChecked,
		"should not proceed to policy file checks",
	)
	assert.Empty(t, result.SyntaxErrors, "should not proceed to syntax checks")
	assert.False(t, result.Valid())
}

func TestValidate_MissingRequiredFile(t *testing.T) {
	ctx := context.Background()
	eval := &evaluator.OPA{}
	s := loadTestCUESchemaInline(t)

	result, err := Validate(
		ctx, "testdata/missing-mapping", eval, []cue.Value{s}, ValidationOptions{},
	)
	require.NoError(t, err)

	assert.Equal(t, []string{"complytime-mapping.json"}, result.MissingFiles)
	assert.Equal(t, 0, result.FilesChecked, "should not proceed to file checks")
	assert.Empty(t, result.SyntaxErrors, "should not proceed to syntax checks")
	assert.False(t, result.Valid())
}

func TestValidate_RequiredFilePresent(t *testing.T) {
	ctx := context.Background()
	eval := &evaluator.OPA{}
	s := loadTestCUESchemaInline(t)

	result, err := Validate(
		ctx, "testdata/valid", eval, []cue.Value{s}, ValidationOptions{},
	)
	require.NoError(t, err)

	assert.Empty(t, result.MissingFiles)
	assert.True(t, result.Valid())
}

func TestCollectFiles(t *testing.T) {
	files, err := collectFiles("testdata/valid", ".rego")
	require.NoError(t, err)
	assert.Len(t, files, 1)

	files, err = collectFiles("testdata/empty", ".rego")
	require.NoError(t, err)
	assert.Len(t, files, 0)
}

func TestValidate_SkipsTestFileSyntaxCheck(t *testing.T) {
	ctx := context.Background()
	eval := &evaluator.OPA{}
	s := loadTestCUESchemaInline(t)

	// The testdata/with-tests directory contains a policy file and a
	// companion _test.rego file that references the policy's deny rule.
	// Without the fix, the test file would fail single-module syntax
	// validation with rego_unsafe_var_error.
	result, err := Validate(
		ctx, "testdata/with-tests", eval, []cue.Value{s},
		ValidationOptions{},
	)
	require.NoError(t, err)

	assert.Equal(t, 1, result.FilesChecked,
		"FilesChecked should exclude test files")
	assert.Empty(t, result.SyntaxErrors,
		"test files should not cause syntax errors")
	assert.True(t, result.Valid())
}

func TestIsTestFile(t *testing.T) {
	assert.True(t, IsTestFile("policy_test.rego", ".rego"))
	assert.False(t, IsTestFile("policy.rego", ".rego"))
	assert.False(t, IsTestFile("test_helper.rego", ".rego"))
}
