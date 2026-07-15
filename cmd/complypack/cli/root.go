// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/complytime/complypack/internal/version"
	"github.com/spf13/cobra"
)

// New creates the root complypack CLI command.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "complypack",
		Short:         "OCI artifact tools for compliance policies and Gemara catalogs",
		Version:       version.ModuleVersion(),
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.AddCommand(mcpCmd())
	cmd.AddCommand(packCmd())
	cmd.AddCommand(pullCmd())
	cmd.AddCommand(cacheCmd())
	cmd.AddCommand(initCmd())

	return cmd
}
