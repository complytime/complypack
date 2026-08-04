// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"strings"

	"github.com/complytime/complypack/internal/config"
	"github.com/spf13/cobra"
)

// configValidateCmd creates the "config validate" subcommand.
func configValidateCmd() *cobra.Command {
	var unknownFields string
	var scopes []string

	cmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate a complypack.yaml configuration file",
		Long: `Validate a complypack.yaml configuration file against the JSON Schema,
structural rules, and scope-specific requirements.

By default, reads complypack.yaml from the current directory.
Optionally pass a path to validate a specific file.

Unknown fields produce warnings by default. Use --unknown-fields=error
to treat them as errors.

Use --scope to validate against specific operation requirements:
  pack   - fields required for complypack pack
  serve  - fields required for complypack mcp serve
  init   - fields required for complypack init (union of pack + serve)
  all    - validate against all scopes (default)

Examples:
  complypack config validate
  complypack config validate path/to/complypack.yaml
  complypack config validate --unknown-fields=error
  complypack config validate --scope pack
  complypack config validate --scope pack --scope serve`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := defaultConfigFilename
			if len(args) > 0 {
				path = args[0]
			}

			// Validate all flag values and report all errors at once
			var errs []string

			validUF := make(map[string]bool, len(config.ValidUnknownFields))
			for _, v := range config.ValidUnknownFields {
				validUF[v] = true
			}
			if !validUF[unknownFields] {
				errs = append(errs, fmt.Sprintf("invalid --unknown-fields value %q: must be one of %s",
					unknownFields, strings.Join(config.ValidUnknownFields, ", ")))
			}

			validS := make(map[string]bool, len(config.ValidScopes))
			for _, v := range config.ValidScopes {
				validS[v] = true
			}
			var invalidScopes []string
			for _, s := range scopes {
				if !validS[s] {
					invalidScopes = append(invalidScopes, s)
				}
			}
			if len(invalidScopes) > 0 {
				errs = append(errs, fmt.Sprintf("invalid --scope values %v: must be one of %s",
					invalidScopes, strings.Join(config.ValidScopes, ", ")))
			}

			if len(errs) > 0 {
				return fmt.Errorf("%s", strings.Join(errs, "; "))
			}

			strict := unknownFields == "error"

			cfg, err := config.LoadConfig(path, strict, cmd.ErrOrStderr())
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			// Run scope-specific validation
			results := cfg.ValidateScopes(scopes)

			allValid := true
			for _, r := range results {
				if r.Valid {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s: ok\n", r.Scope)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s: FAIL - %s\n", r.Scope, r.Error)
					allValid = false
				}
			}

			if !allValid {
				return fmt.Errorf("validation failed: config is not valid for all requested scopes")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s is valid\n", path)
			return nil
		},
	}

	cmd.Flags().StringVar(&unknownFields, "unknown-fields", "warn",
		fmt.Sprintf("How to handle unknown config fields: %s", strings.Join(config.ValidUnknownFields, " or ")))
	cmd.Flags().StringSliceVar(&scopes, "scope", nil,
		fmt.Sprintf("Validation scope: %s (default: all)", strings.Join(config.ValidScopes, ", ")))

	return cmd
}
