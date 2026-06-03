// SPDX-License-Identifier: Apache-2.0

package cli

import "github.com/spf13/cobra"

// New creates the root complypack CLI command.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "complypack",
		Short:         "OCI artifact tools for compliance policies and Gemara catalogs",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.AddCommand(mcpCmd())
	cmd.AddCommand(packCmd())

	return cmd
}
