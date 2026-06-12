// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"testing"

	"github.com/complytime/complypack/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackCommand(t *testing.T) {
	root := New()

	t.Run("command exists", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"pack"})
		require.NoError(t, err)
		assert.Equal(t, "pack", cmd.Name())
	})

	t.Run("has flags", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"pack"})
		require.NoError(t, err)

		assert.NotNil(t, cmd.Flags().Lookup("config"))
		assert.NotNil(t, cmd.Flags().Lookup("plain-http"))
	})

	t.Run("requires exactly 2 args", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"pack"})
		require.NoError(t, err)

		err = cmd.Args(cmd, []string{})
		assert.Error(t, err)

		err = cmd.Args(cmd, []string{"dir", "ref"})
		assert.NoError(t, err)
	})
}

func TestRunPrePackValidation_UnregisteredEvaluator(t *testing.T) {
	cfg := &config.ComplyPackConfig{
		ID:          "io.complytime.test",
		EvaluatorID: "ampel",
		Version:     "1.0.0",
	}

	err := runPrePackValidation(context.Background(), cfg, t.TempDir(), false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `evaluator "ampel" has no registered validator`)
	assert.Contains(t, err.Error(), "--skip-validation")
	assert.NotContains(t, err.Error(), "not found")
}
