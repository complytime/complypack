// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sort"
	"time"
	"cuelang.org/go/cue"
	"github.com/complytime/complypack/internal/cache"
	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/coverage"
	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/internal/packer"
	"github.com/complytime/complypack/internal/pipeline"
	"github.com/complytime/complypack/internal/prepack"
	"github.com/complytime/complypack/internal/registry"
	"github.com/complytime/complypack/internal/schema"
	"github.com/complytime/complypack/pkg/complypack"
	"github.com/complytime/complypack/schemas"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
)

// packResolveTimeout bounds the aggregate resolution of all declared Gemara
// sources during pack so that a large, slow, or hostile source cannot hang
// the command indefinitely (CWE-400). Exceeding it fails the pack closed.
const packResolveTimeout = 5 * time.Minute

func packCmd() *cobra.Command {
	var (
		configPath     string
		plainHTTP      bool
		skipValidation bool
		skipTests      bool
		cacheDir       string
	)

	cmd := &cobra.Command{
		Use:   "pack <content-dir> <oci-reference>",
		Short: "Pack policy content into a ComplyPack OCI artifact",
		Long: `Pack a directory of policy content into a ComplyPack OCI artifact
and push it to an OCI registry.

Reads evaluator-id, version, and gemara sources from complypack.yaml.
The declared gemara sources are resolved and the resulting policy
provenance (policy IDs and the Gemara artifacts they import) is
recorded in the artifact's config blob. The content directory is
archived as a tar.gz and stored as the artifact's opaque content layer.

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
			cfg, err := config.LoadConfig(configPath, true, os.Stderr)
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

			// Resolve Gemara sources and record their provenance so a
			// consumer can tell which policies this pack implements.
			// Fail-closed: an unresolvable source aborts the pack.
			provenance, err := resolveProvenance(ctx, cfg, cacheDir)
			if err != nil {
				return err
			}

			// Build complypack config from complypack.yaml
			packCfg := complypack.Config{
				ID:          cfg.ID,
				EvaluatorID: cfg.EvaluatorID,
				Version:     cfg.Version,
				Source:      provenance,
			}

			// Create tarball from content directory, excluding test
			// files.  Test files follow the *_test.<ext> convention
			// and should not be shipped in the published artifact.
			log.Printf("Packing %s...", contentDir)
			var tarOpts []packer.TarOption
			reg := evaluator.DefaultRegistry()
			if eval, evalErr := reg.Get(cfg.EvaluatorID); evalErr == nil {
				ext := eval.FileExtension()
				tarOpts = append(tarOpts, packer.WithExclude(func(relPath string) bool {
					return prepack.IsTestFile(relPath, ext)
				}))
			} else {
				slog.Warn("could not resolve evaluator for test-file exclusion",
					"evaluator", cfg.EvaluatorID, "error", evalErr)
			}
			content, err := packer.TarGzipDir(contentDir, tarOpts...)
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
	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", cache.CacheDirHelp)

	return cmd
}

// resolveProvenance resolves the Gemara sources declared in cfg and maps
// them to the provenance records recorded in the pack's config blob.
//
// It hard-fails (fail-closed) when any declared source cannot be loaded or
// resolved, naming every offending source (credential-sanitized). A config
// with no declared sources yields nil provenance. Sources that load and
// merge cleanly but resolve to no policy yield empty provenance without
// error. Resolution runs under a bounded context (CWE-400).
func resolveProvenance(ctx context.Context, cfg *config.ComplyPackConfig, cacheDir string) ([]complypack.Provenance, error) {
	if len(cfg.Gemara.Sources) == 0 {
		return nil, nil
	}

	resolvedCacheDir, err := cache.ResolveDir(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cache directory: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, packResolveTimeout)
	defer cancel()

	// Resolution can fetch remote sources and run up to packResolveTimeout;
	// log progress so the operator is not left staring at a silent command.
	slog.Info("Resolving gemara sources", "count", len(cfg.Gemara.Sources))
	result, err := pipeline.LoadAndResolve(ctx, cfg.Gemara.Sources, resolvedCacheDir)
	if err != nil {
		return nil, fmt.Errorf("resolving gemara sources: %w", err)
	}
	slog.Info("Resolved gemara sources",
		"sources", len(cfg.Gemara.Sources), "policies", len(result.Resolved))

	return pipeline.BuildProvenance(result.Resolved, result.PolicySources), nil
}

// runPrePackValidation runs the 3-stage validation pipeline before packing.
func runPrePackValidation(ctx context.Context, cfg *config.ComplyPackConfig, contentDir string, skipTests bool) error {
	// Resolve evaluator
	reg := evaluator.DefaultRegistry()
	eval, err := reg.Get(cfg.EvaluatorID)
	if err != nil {
		if errors.Is(err, evaluator.ErrNotFound) {
			return fmt.Errorf(
				"evaluator %q has no registered validator; use --skip-validation to pack without pre-pack checks",
				cfg.EvaluatorID,
			)
		}
		return fmt.Errorf("evaluator %q: %w", cfg.EvaluatorID, err)
	}

	// Load CUE schemas for contract validation
	index, err := schemas.LoadIndex()
	if err != nil {
		return fmt.Errorf("loading schema index: %w", err)
	}

	var cueSchemas []cue.Value
	if len(cfg.Schemas) > 0 {
		schemaReg := schema.DefaultRegistry()
		for _, ref := range cfg.Schemas {
			source := schemas.ResolveSource(ref, index)

			s, err := schemaReg.Load(ctx, source, ref.Platform)
			if err != nil {
				if source == "" {
					slog.Warn("no schema available for platform, skipping",
						"platform", ref.Platform, "error", err)
					continue
				}
				return fmt.Errorf("loading CUE schema for %s: %w", ref.Platform, err)
			}
			cueSchemas = append(cueSchemas, s.CUE)
		}
	}

	log.Printf("Validating policies in %s...", contentDir)
	result, err := prepack.Validate(ctx, contentDir, eval, cueSchemas, prepack.ValidationOptions{
		SkipTests: skipTests,
	})
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if len(result.MissingFiles) > 0 {
		log.Printf("  missing required files: %d", len(result.MissingFiles))
		for _, f := range result.MissingFiles {
			log.Printf("    %s", f)
		}
		return fmt.Errorf(
			"content directory is missing required file(s) for evaluator %q; "+
				"run the assessment skill to generate them",
			cfg.EvaluatorID,
		)
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

		// Compute per-requirement test attribution
		perReq, attrErr := coverage.AttributeTests(contentDir, "", result.TestResults)
		if attrErr != nil {
			log.Printf("  WARNING: test attribution failed: %v", attrErr)
		} else if len(perReq) > 0 {
			// Sort requirement IDs for deterministic output
			reqIDs := make([]string, 0, len(perReq))
			for id := range perReq {
				reqIDs = append(reqIDs, id)
			}
			sort.Strings(reqIDs)
			for _, id := range reqIDs {
				log.Printf("    %s: %s", id, perReq[id])
			}
		}

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
