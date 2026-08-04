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
	})

	t.Run("invalid source returns error", func(t *testing.T) {
		sources := []config.GemaraSourceEntry{
			{Source: "file:///nonexistent/path", PlainHTTP: false},
		}
		result, err := pipeline.LoadAndResolve(
			context.Background(), sources, "",
		)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to load artifacts")
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
