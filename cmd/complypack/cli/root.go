// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/complytime/complypack/internal/version"
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
	cmd.AddCommand(versionCmd())

	return cmd
}

func versionCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(_ *cobra.Command, _ []string) error {
			v := version.Get()
			if jsonOutput {
				out, err := json.MarshalIndent(v, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			}
			fmt.Printf("complypack %s (commit: %s, built: %s)\n", v.Version, v.Commit, v.BuildDate)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
	return cmd
}
