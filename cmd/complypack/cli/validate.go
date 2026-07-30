// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/complytime/complypack/internal/config"
	"github.com/spf13/cobra"
)

func validateCmd() *cobra.Command {
	var strict bool

	cmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate a complypack.yaml configuration file",
		Long: `Validate a complypack.yaml configuration file against the JSON Schema
and structural rules.

By default, reads complypack.yaml from the current directory.
Optionally pass a path to validate a specific file.

Unknown fields produce warnings by default. Use --strict to treat
them as errors.

Examples:
  complypack validate
  complypack validate path/to/complypack.yaml
  complypack validate --strict`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := defaultConfigFilename
			if len(args) > 0 {
				path = args[0]
			}

			_, err := config.LoadConfig(path, strict, cmd.ErrOrStderr())
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s is valid\n", path)
			return nil
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "Treat unknown config fields as errors")

	return cmd
}
