// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"log"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/internal/mcp"
	"github.com/complytime/complypack/internal/packer"
	"github.com/complytime/complypack/internal/prepack"
	"github.com/complytime/complypack/internal/registry"
	"github.com/complytime/complypack/pkg/complypack"
	"github.com/complytime/complypack/schemas"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
)

func packCmd() *cobra.Command {
	var (
		configPath     string
		plainHTTP      bool
		skipValidation bool
		skipTests      bool
	)

	cmd := &cobra.Command{
		Use:   "pack <content-dir> <oci-reference>",
		Short: "Pack policy content into a ComplyPack OCI artifact",
		Long: `Pack a directory of policy content into a ComplyPack OCI artifact
and push it to an OCI registry.

Reads evaluator-id, version, and gemara source from complypack.yaml.
The content directory is archived as a tar.gz and stored as the
artifact's opaque content layer.

By default, policies are validated before packing:
  1. Syntax checking
  2. Contract validation against platform schema
  3. Policy test execution

Use --skip-validation to bypass all checks, or --skip-tests to
skip only test execution.

Examples:
  complypack pack policy/ ghcr.io/org/my-policies:v1.0.0
  complypack pack policy/ localhost:5001/test:latest --plain-http
  complypack pack policy/ ghcr.io/org/policies:v1 --skip-tests`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			contentDir := args[0]
			ref := args[1]

			// Load config
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			if err := cfg.ValidateForPack(); err != nil {
				return fmt.Errorf("config validation: %w", err)
			}

			// Run pre-pack validation unless skipped
			if !skipValidation {
				if err := runPrePackValidation(ctx, cfg, contentDir, skipTests); err != nil {
					return err
				}
			}

			// Build complypack config from complypack.yaml
			packCfg := complypack.Config{
				ID:          cfg.ID,
				EvaluatorID: cfg.EvaluatorID,
				Version:     cfg.Version,
			}

			// Create tarball from content directory
			log.Printf("Packing %s...", contentDir)
			content, err := packer.TarGzipDir(contentDir)
			if err != nil {
				return fmt.Errorf("creating archive: %w", err)
			}

			// Pack into OCI artifact
			store := memory.New()
			desc, err := complypack.Pack(ctx, store, packCfg, content)
			if err != nil {
				return fmt.Errorf("packing artifact: %w", err)
			}

			// Tag
			tag := registry.ParseTag(ref)
			if err := store.Tag(ctx, desc, tag); err != nil {
				return fmt.Errorf("tagging artifact: %w", err)
			}

			// Push to registry
			credFunc, err := registry.NewCredentialFunc()
			if err != nil {
				return fmt.Errorf("loading credentials: %w", err)
			}

			repo, err := registry.NewRepository(ref, credFunc, plainHTTP)
			if err != nil {
				return fmt.Errorf("creating repository: %w", err)
			}

			log.Printf("Pushing to %s...", ref)
			_, err = oras.Copy(ctx, store, tag, repo, tag, oras.DefaultCopyOptions)
			if err != nil {
				return fmt.Errorf("pushing artifact: %w", err)
			}

			log.Printf("Published %s", ref)
			log.Printf("  evaluator-id: %s", packCfg.EvaluatorID)
			log.Printf("  version:      %s", packCfg.Version)
			log.Printf("  digest:       %s", desc.Digest)

			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "complypack.yaml", "Path to complypack.yaml")
	cmd.Flags().BoolVar(&plainHTTP, "plain-http", false, "Use HTTP instead of HTTPS for the registry")
	cmd.Flags().BoolVar(&skipValidation, "skip-validation", false, "Skip all pre-pack validation")
	cmd.Flags().BoolVar(&skipTests, "skip-tests", false, "Run syntax and contract checks but skip test execution")

	return cmd
}

// runPrePackValidation runs the 3-stage validation pipeline before packing.
func runPrePackValidation(ctx context.Context, cfg *config.ComplyPackConfig, contentDir string, skipTests bool) error {
	// Resolve evaluator
	reg := evaluator.DefaultRegistry()
	eval, err := reg.Get(cfg.EvaluatorID)
	if err != nil {
		return fmt.Errorf("evaluator %q not found: %w", cfg.EvaluatorID, err)
	}

	// Load CUE schema for contract validation
	var cueSchema cue.Value
	if len(cfg.Schemas) > 0 {
		ref := cfg.Schemas[0]
		source := ref.Source
		if source == "" && ref.Path != "" {
			source = "file://" + ref.Path
		}

		if source != "" {
			parsed, parseErr := mcp.ParseSchemaSource(source)
			if parseErr != nil {
				return fmt.Errorf("invalid schema source for %s: %w", ref.Platform, parseErr)
			}
			cueSchema, err = mcp.LoadCUEFromSource(ctx, parsed, ref.Platform)
			if err != nil {
				return fmt.Errorf("loading CUE schema for %s: %w", ref.Platform, err)
			}
		} else {
			cueSchema, err = loadEmbeddedCUE(ref.Platform)
			if err != nil {
				return fmt.Errorf("loading embedded CUE schema for %s: %w", ref.Platform, err)
			}
		}
	}

	log.Printf("Validating policies in %s...", contentDir)
	result, err := prepack.Validate(ctx, contentDir, eval, cueSchema, prepack.ValidationOptions{
		SkipTests: skipTests,
	})
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	log.Printf("  files checked: %d", result.FilesChecked)

	if len(result.SyntaxErrors) > 0 {
		log.Printf("  syntax errors: %d", len(result.SyntaxErrors))
		for _, e := range result.SyntaxErrors {
			log.Printf("    %s: %s", e.File, e.Error)
		}
		return fmt.Errorf("validation failed: %d syntax error(s)", len(result.SyntaxErrors))
	}

	if len(result.ContractViolations) > 0 {
		log.Printf("  contract violations: %d", len(result.ContractViolations))
		for _, v := range result.ContractViolations {
			log.Printf("    %s: %s", v.Location, v.Path)
		}
		return fmt.Errorf("validation failed: %d contract violation(s)", len(result.ContractViolations))
	}

	if result.TestResults != nil {
		log.Printf("  tests: %d passed, %d failed (of %d)",
			result.TestResults.Passed, result.TestResults.Failed, result.TestResults.Total)
		if result.TestResults.Failed > 0 {
			for _, e := range result.TestResults.Errors {
				log.Printf("    %s", e)
			}
			return fmt.Errorf("validation failed: %d test(s) failed", result.TestResults.Failed)
		}
	} else if result.TestsSkipped {
		log.Printf("  tests: skipped")
	}

	log.Printf("Validation passed.")
	return nil
}

// loadEmbeddedCUE loads a CUE schema from embedded files.
func loadEmbeddedCUE(platform string) (cue.Value, error) {
	data, err := schemas.GetBuiltInCUESchema(platform)
	if err != nil {
		return cue.Value{}, err
	}

	ctx := cuecontext.New()
	val := ctx.CompileBytes(data)
	if val.Err() != nil {
		return cue.Value{}, val.Err()
	}
	return val, nil
}
