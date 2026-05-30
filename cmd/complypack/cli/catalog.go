// SPDX-License-Identifier: Apache-2.0

package cli

import "github.com/spf13/cobra"

// catalogCmd creates the catalog parent command.
func catalogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Work with Gemara catalogs",
	}

	cmd.AddCommand(catalogPullCmd())

	return cmd
}
