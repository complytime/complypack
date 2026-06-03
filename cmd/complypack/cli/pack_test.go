// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"testing"

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
