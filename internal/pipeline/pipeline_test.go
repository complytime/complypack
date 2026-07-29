// SPDX-License-Identifier: Apache-2.0

package pipeline_test

import (
	"context"
	"testing"

	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
}
