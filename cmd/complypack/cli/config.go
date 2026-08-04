// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/spf13/cobra"
)

// configCmd creates the "config" command group.
func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management commands",
		Long:  "Commands for managing complypack.yaml configuration files.",
	}

	cmd.AddCommand(configValidateCmd())

	return cmd
}
