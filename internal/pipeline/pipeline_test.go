// SPDX-License-Identifier: Apache-2.0

package pipeline_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// policyNoImportsYAML is a valid Gemara policy that imports no catalogs
// or guidance. ResolvePolicy resolves it to a valid empty ResolvedPolicy.
const policyNoImportsYAML = `metadata:
  id: standalone-policy
  type: Policy
  gemara-version: "1.0.0"
`

// policyUnresolvableImportYAML is a valid Gemara policy that loads and
// classifies successfully but imports a catalog whose reference-id has no
// matching loaded artifact, so ResolvePolicy fails. It exercises the
// resolve-error propagation path in LoadAndResolve.
const policyUnresolvableImportYAML = `metadata:
  id: unresolvable-policy
  type: Policy
  gemara-version: "1.0.0"
imports:
  catalogs:
    - reference-id: missing-catalog
`

func TestLoadAndResolve(t *testing.T) {
	t.Run("empty sources returns empty result", func(t *testing.T) {
		result, err := pipeline.LoadAndResolve(
			context.Background(), nil, "",
		)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Artifacts)
		assert.Empty(t, result.Resolved)
		assert.Empty(t, result.Artifacts.Catalogs)
		assert.Empty(t, result.Artifacts.Policies)
	})

	t.Run("resolves policy with no catalogs or guidance", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "policy.yaml")
		err := os.WriteFile(path, []byte(policyNoImportsYAML), 0600)
		require.NoError(t, err)

		sources := []config.GemaraSourceEntry{
			{Source: path, PlainHTTP: false},
		}
		result, err := pipeline.LoadAndResolve(
			context.Background(), sources, "",
		)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Empty(t, result.Artifacts.Catalogs)
		assert.Empty(t, result.Artifacts.Guidance)
		assert.Contains(t, result.Artifacts.Policies, "standalone-policy")
		assert.Contains(t, result.Resolved, "standalone-policy",
			"policy must be resolved even when no catalogs or guidance are loaded")
		require.NotNil(t, result.Resolved["standalone-policy"])
		assert.Equal(t, path, result.PolicySources["standalone-policy"],
			"PolicySources must record the source that provided the policy")
	})

	t.Run("invalid source returns error naming the source", func(t *testing.T) {
		sources := []config.GemaraSourceEntry{
			{Source: "file:///nonexistent/path", PlainHTTP: false},
		}
		result, err := pipeline.LoadAndResolve(
			context.Background(), sources, "",
		)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "source file:///nonexistent/path")
	})

	t.Run("multiple bad sources are all named in one joined error", func(t *testing.T) {
		sources := []config.GemaraSourceEntry{
			{Source: "file:///nonexistent/one"},
			{Source: "file:///nonexistent/two"},
		}
		result, err := pipeline.LoadAndResolve(
			context.Background(), sources, "",
		)
		require.Error(t, err)
		assert.Nil(t, result)
		// Both failed sources must appear in the single joined error,
		// proving the loop does not return on the first failure.
		assert.Contains(t, err.Error(), "source file:///nonexistent/one")
		assert.Contains(t, err.Error(), "source file:///nonexistent/two")
	})

	t.Run("one good and one bad source names only the bad and does not partially resolve", func(t *testing.T) {
		good := "file://../../examples/gemara/control-catalog.yaml"
		sources := []config.GemaraSourceEntry{
			{Source: good},
			{Source: "file:///nonexistent/bad"},
		}
		result, err := pipeline.LoadAndResolve(
			context.Background(), sources, "",
		)
		require.Error(t, err)
		// A load failure aborts the whole operation: no partial result.
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "source file:///nonexistent/bad")
		assert.NotContains(t, err.Error(), "source "+good)
	})

	t.Run("merge conflict across sources is reported as a batched, named error", func(t *testing.T) {
		// Loading the same artifact twice yields a duplicate-ID merge
		// failure. This exercises the merge-error branch (not just the
		// load-error branch) and proves merge failures are collected into
		// the joined error and named by their (redacted) source.
		dup := "file://../../examples/gemara/control-catalog.yaml"
		sources := []config.GemaraSourceEntry{
			{Source: dup},
			{Source: dup},
		}
		result, err := pipeline.LoadAndResolve(
			context.Background(), sources, "",
		)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "duplicate artifact id",
			"a merge conflict must surface as a merge error")
		assert.Contains(t, err.Error(), "source "+dup,
			"the merge-failing source must be named")
	})

	t.Run("cancelled context aborts resolution and returns an error", func(t *testing.T) {
		// The pack timeout (CWE-400 bound) relies on context cancellation
		// propagating through LoadAndResolve into the source loader. An
		// already-cancelled context must fail closed with no result.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		sources := []config.GemaraSourceEntry{
			{Source: "oci://ghcr.io/complytime/nonexistent:v1"},
		}
		result, err := pipeline.LoadAndResolve(ctx, sources, "")
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("credential-bearing failing source is named without leaking the secret", func(t *testing.T) {
		// Proves RedactCredentials is applied to the source label on the
		// batch-error path (CWE-209): the failure names the redacted source
		// and the embedded password never appears in the joined error.
		sources := []config.GemaraSourceEntry{
			{Source: "https://user:supersecret@registry.invalid/org/repo:tag"}, //nolint:gosec // test fixture, not real credentials
		}
		result, err := pipeline.LoadAndResolve(
			context.Background(), sources, "",
		)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.NotContains(t, err.Error(), "supersecret",
			"embedded credential must not appear in the error")
		assert.Contains(t, err.Error(), "source https://registry.invalid/org/repo:tag",
			"failed source is named in its redacted form")
	})

	t.Run("resolve failure returns error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "policy.yaml")
		err := os.WriteFile(path, []byte(policyUnresolvableImportYAML), 0600)
		require.NoError(t, err)

		sources := []config.GemaraSourceEntry{
			{Source: path, PlainHTTP: false},
		}
		result, err := pipeline.LoadAndResolve(
			context.Background(), sources, "",
		)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to resolve effective policy")
	})
}
