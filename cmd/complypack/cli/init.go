// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/complytime/complypack/internal/config"
	pkgversion "github.com/complytime/complypack/internal/version"
	"github.com/complytime/complypack/schemas"
	"github.com/complytime/complypack/schemas/jsonschema"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const defaultConfigFilename = "complypack.yaml"

func initCmd() *cobra.Command {
	var (
		schemaFlags      []string
		sourceFlags      []string
		evaluatorID      string
		id               string
		version          string
		force            bool
		output           string
		strict           bool
		allowCredentials bool
		createParents    bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a complypack.yaml configuration file",
		Long: `Generate a complypack.yaml configuration file interactively or from flags.

When --schema and --source flags are provided, the config is generated
directly from flag values (non-interactive mode).

Without content flags, an interactive form prompts for pack identity,
platform schemas (from the built-in schema index), and Gemara sources.

Examples:
  # Interactive — prompts for platforms and sources
  complypack init

  # From flags — no prompts
  complypack init \
    --schema kubernetes-deployment \
    --schema ci-github-actions \
    --source oci://ghcr.io/org/catalog:latest \
    --evaluator-id opa \
    --id io.complytime.my-pack \
    --version 0.1.0`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If output targets a directory, append the default filename.
			if output != "" && os.IsPathSeparator(output[len(output)-1]) {
				output = filepath.Join(output, defaultConfigFilename)
			} else if info, err := os.Stat(output); err == nil && info.IsDir() {
				output = filepath.Join(output, defaultConfigFilename)
			}

			interactive := len(schemaFlags) == 0 && len(sourceFlags) == 0

			if interactive {
				if !term.IsTerminal(int(os.Stdin.Fd())) { //nolint:gosec // Fd() returns a valid file descriptor; truncation is not a real risk
					return fmt.Errorf("interactive mode requires a terminal; use --schema and --source flags for non-interactive use")
				}

				defaults := initOptions{
					id:          id,
					evaluatorID: evaluatorID,
					version:     version,
				}
				opts, err := runInteractiveInit(cmd.Context(), output, defaults)
				if err != nil {
					return err
				}

				id = opts.id
				evaluatorID = opts.evaluatorID
				version = opts.version
				sourceFlags = opts.sources
				schemaFlags = opts.schemas
				force = opts.force
				createParents = opts.createParents
			}

			if err := config.CheckCredentialURIs(cmd.ErrOrStderr(), sourceFlags, allowCredentials); err != nil {
				return err
			}

			cfg, err := buildInitConfig(id, evaluatorID, version, sourceFlags, schemaFlags)
			if err != nil {
				return err
			}

			data, err := config.MarshalConfigYAML(cfg, pkgversion.ModuleVersion(), time.Now().UTC().Format("2006-01-02"))
			if err != nil {
				return fmt.Errorf("marshaling config: %w", err)
			}

			dumpOnError := func(err error) error {
				fmt.Fprintf(cmd.ErrOrStderr(), "\n--- generated config (recovery) ---\n%s--- end generated config ---\n", data)
				return err
			}

			warnings, err := jsonschema.ValidateConfig(data, strict)
			if err != nil {
				return dumpOnError(fmt.Errorf("config validation: %w", err))
			}
			for _, w := range warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: %s\n", w)
			}

			if err := cfg.ValidateForInit(); err != nil {
				return dumpOnError(err)
			}

			if err := config.WriteConfigFile(output, data, force, createParents); err != nil {
				return dumpOnError(err)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %s\n", output)
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&schemaFlags, "schema", nil, "Platform schema (repeatable, e.g. kubernetes-deployment or ci-github-actions=cue://...)")
	cmd.Flags().StringArrayVar(&sourceFlags, "source", nil, "Gemara OCI source (repeatable, e.g. oci://ghcr.io/org/catalog:v1)")
	cmd.Flags().StringVar(&evaluatorID, "evaluator-id", "opa", "Evaluator plugin ID")
	cmd.Flags().StringVar(&id, "id", "", "Pack identifier (e.g. io.complytime.my-pack)")
	cmd.Flags().StringVar(&version, "version", "0.1.0", "Pack version")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config file without prompting")
	cmd.Flags().StringVarP(&output, "output", "o", defaultConfigFilename, "Output file path")
	cmd.Flags().BoolVar(&strict, "strict", false, "Treat unknown config fields as errors")
	cmd.Flags().BoolVar(&allowCredentials, "allow-credentials", false, "Allow source URIs with embedded credentials (not recommended)")
	cmd.Flags().BoolVarP(&createParents, "parents", "p", false, "Create parent directories for the output path if they do not exist")

	return cmd
}

// buildInitConfig creates a ComplyPackConfig from init command parameters.
func buildInitConfig(id, evaluatorID, version string, sources, schemas []string) (*config.ComplyPackConfig, error) {
	gemaraSources, err := parseSourceFlags(sources)
	if err != nil {
		return nil, err
	}

	schemaRefs, err := parseSchemaFlags(schemas)
	if err != nil {
		return nil, err
	}

	return config.BuildConfig(id, evaluatorID, version, gemaraSources, schemaRefs), nil
}

type initOptions struct {
	id            string
	evaluatorID   string
	version       string
	sources       []string
	schemas       []string
	force         bool
	createParents bool
}

// platformLabel generates a display label for a platform name,
// grouping by known prefixes.
func platformLabel(platform string) string {
	switch {
	case strings.HasPrefix(platform, "ci-"):
		return "CI / " + strings.TrimPrefix(platform, "ci-")
	case strings.HasPrefix(platform, "kubernetes-"):
		return "Kubernetes / " + strings.TrimPrefix(platform, "kubernetes-")
	default:
		return platform
	}
}

// buildPlatformOptions creates huh.Option entries for all platforms in the schema index.
func buildPlatformOptions() []huh.Option[string] {
	platforms := schemas.Platforms()
	opts := make([]huh.Option[string], len(platforms))
	for i, p := range platforms {
		opts[i] = huh.NewOption(platformLabel(p), p)
	}
	return opts
}

// validatePackID checks that s is a non-empty, valid reverse-domain pack ID.
func validatePackID(s string) error {
	if s == "" {
		return fmt.Errorf("id is required")
	}
	if !jsonschema.IDPattern().MatchString(s) {
		return fmt.Errorf("must be reverse-domain notation (e.g. io.complytime.my-pack)")
	}
	return nil
}

// validateVersion checks that s is a non-empty, valid semver version.
func validateVersion(s string) error {
	if s == "" {
		return fmt.Errorf("version is required")
	}
	if !jsonschema.VersionPattern().MatchString(s) {
		return fmt.Errorf("must be semver (e.g. 1.0.0, 2.1.0-rc.1)")
	}
	return nil
}

// validatePlatformSelection checks that at least one platform is selected.
func validatePlatformSelection(selected []string) error {
	if len(selected) == 0 {
		return fmt.Errorf("select at least one platform")
	}
	return nil
}

// validateGemaraSource checks that s is a non-empty source URI.
func validateGemaraSource(s string) error {
	if s == "" {
		return fmt.Errorf("at least one gemara source is required")
	}
	return nil
}

// runInteractiveInit runs the interactive form and returns collected values.
func runInteractiveInit(ctx context.Context, outputPath string, defaults initOptions) (*initOptions, error) {
	var (
		id          = defaults.id
		evaluatorID = defaults.evaluatorID
		version     = defaults.version
		platforms   []string
		source      string
	)

	identityGroup := huh.NewGroup(
		huh.NewInput().
			Title("Pack ID").
			Description("Globally unique identifier (e.g. io.complytime.my-pack)").
			Placeholder("io.example.my-pack").
			Value(&id).
			Validate(validatePackID),
		huh.NewInput().
			Title("Version").
			Description("ComplyPack artifact version").
			Value(&version).
			Validate(validateVersion),
		huh.NewSelect[string]().
			Title("Evaluator").
			Description("Policy evaluator plugin").
			Options(
				huh.NewOption("OPA (Open Policy Agent)", "opa"),
			).
			Value(&evaluatorID),
	).Title("Pack Identity")

	schemaGroup := huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Platform Schemas").
			Description("Select platforms to include (space to toggle, enter to confirm)").
			Options(buildPlatformOptions()...).
			Value(&platforms).
			Filterable(true).
			Height(15).
			Validate(validatePlatformSelection),
	).Title("Schemas")

	sourceGroup := huh.NewGroup(
		huh.NewInput().
			Title("Gemara Source").
			Description("OCI URI for compliance catalog (e.g. oci://ghcr.io/org/catalog:v1)").
			Placeholder("oci://ghcr.io/org/catalog:latest").
			Value(&source).
			Validate(validateGemaraSource),
	).Title("Gemara Source")

	form := huh.NewForm(identityGroup, schemaGroup, sourceGroup)

	if err := form.RunWithContext(ctx); err != nil {
		return nil, fmt.Errorf("interactive form: %w", err)
	}

	opts := &initOptions{
		id:          id,
		evaluatorID: evaluatorID,
		version:     version,
		schemas:     platforms,
	}

	if source != "" {
		opts.sources = []string{source}
	}

	// Check whether parent directories need to be created
	dir := filepath.Dir(outputPath)
	if dir != "." && dir != "/" {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			var confirm bool
			confirmErr := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title(fmt.Sprintf("Directory %s does not exist. Create it?", dir)).
						Value(&confirm),
				),
			).RunWithContext(ctx)
			if confirmErr != nil {
				return nil, fmt.Errorf("confirmation prompt: %w", confirmErr)
			}
			if !confirm {
				return nil, fmt.Errorf("aborted: directory %s not created", dir)
			}
			opts.createParents = true
		}
	}

	// Check for overwrite
	if _, err := os.Lstat(outputPath); err == nil {
		var confirm bool
		confirmErr := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("%s already exists. Overwrite?", outputPath)).
					Value(&confirm),
			),
		).RunWithContext(ctx)
		if confirmErr != nil {
			return nil, fmt.Errorf("confirmation prompt: %w", confirmErr)
		}
		if !confirm {
			return nil, fmt.Errorf("aborted: %s not overwritten", outputPath)
		}
		opts.force = true
	}

	return opts, nil
}
