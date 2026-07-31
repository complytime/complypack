// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/complytime/complypack/internal/cache"
	"github.com/complytime/complypack/internal/requirement"
	"github.com/spf13/cobra"
)

func requirementsCmd() *cobra.Command {
	var (
		catalogName string
		controlID   string
		configPath  string
		cacheDir    string
		format      string
		sources     []string
		scope       []string
	)

	cmd := &cobra.Command{
		Use:   "requirements",
		Short: "Extract assessment requirements from a catalog or policy",
		Long: `Extract assessment requirements with structured parameters
from assessment plans. Supports filtering by control ID and
applicability scope.

Output formats:
  human  Structured text with headers and indentation (default)
  text   Plain text suitable for CI/grep
  json   Structured JSON object

Examples:
  complypack requirements --catalog my-catalog \
    --source oci://ghcr.io/org/catalog:v1
  complypack requirements --catalog my-catalog \
    --control CTL-001 --format json
  complypack requirements --catalog my-catalog \
    --scope maturity-1 --scope maturity-2`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedFormat, err := resolveFormat(format)
			if err != nil {
				return err
			}
			return runRequirements(cmd, requirementsRunParams{
				catalogName: catalogName,
				controlID:   controlID,
				configPath:  configPath,
				cacheDir:    cacheDir,
				format:      resolvedFormat,
				sources:     sources,
				scope:       scope,
				stdout:      cmd.OutOrStdout(),
			})
		},
	}

	cmd.Flags().StringVar(
		&catalogName, "catalog", "",
		"Catalog or policy name to extract from (required)",
	)
	cmd.Flags().StringVar(
		&controlID, "control", "",
		"Filter requirements to a specific control ID",
	)
	cmd.Flags().StringVarP(
		&configPath, "config", "c", "",
		"Path to complypack.yaml config file",
	)
	cmd.Flags().StringVar(
		&cacheDir, "cache-dir", "", cache.CacheDirHelp,
	)
	cmd.Flags().StringVarP(
		&format, "format", "f", "",
		"Output format: human, text, or json (default: auto)",
	)
	cmd.Flags().StringArrayVar(
		&sources, "source", nil,
		"Gemara OCI source (repeatable)",
	)
	cmd.Flags().StringArrayVar(
		&scope, "scope", nil,
		"Filter by applicability group (repeatable)",
	)

	_ = cmd.MarkFlagRequired("catalog")

	_ = cmd.RegisterFlagCompletionFunc(
		"format",
		func(_ *cobra.Command, _ []string, _ string) (
			[]string, cobra.ShellCompDirective,
		) {
			return []string{formatHuman, formatText, formatJSON},
				cobra.ShellCompDirectiveNoFileComp
		},
	)

	return cmd
}

type requirementsRunParams struct {
	catalogName string
	controlID   string
	configPath  string
	cacheDir    string
	format      string
	sources     []string
	scope       []string
	stdout      io.Writer
}

func runRequirements(
	cmd *cobra.Command,
	params requirementsRunParams,
) error {
	ctx := cmd.Context()

	loadResult, err := loadArtifacts(
		ctx, params.sources, params.configPath, params.cacheDir,
	)
	if err != nil {
		return err
	}

	rp, err := findResolvedPolicyCLI(
		loadResult, params.catalogName,
	)
	if err != nil {
		return err
	}

	results := requirement.ExtractAssessmentRequirements(
		rp, params.controlID, params.scope,
	)

	switch params.format {
	case formatJSON:
		return writeRequirementsJSON(params.stdout, results)
	case formatText:
		return writeRequirementsText(
			params.stdout, params.catalogName, results,
		)
	case formatHuman:
		return writeRequirementsHuman(
			params.stdout, params.catalogName, results,
		)
	default:
		return fmt.Errorf(
			"unknown format %q", params.format,
		)
	}
}

func writeRequirementsJSON(
	w io.Writer,
	results []requirement.AssessmentRequirementInfo,
) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func writeRequirementsText(
	w io.Writer,
	catalogName string,
	results []requirement.AssessmentRequirementInfo,
) error {
	fmt.Fprintf(w, "Requirements: %s\n", catalogName)
	fmt.Fprintln(w, strings.Repeat("=", 50))
	fmt.Fprintf(w, "  Count: %d\n", len(results))

	for _, r := range results {
		fmt.Fprintf(
			w, "\n  %s (control: %s)\n", r.ID, r.ControlID,
		)
		fmt.Fprintf(w, "    %s\n", r.Text)
		if len(r.Applicability) > 0 {
			fmt.Fprintf(
				w, "    Applicability: %s\n",
				strings.Join(r.Applicability, ", "),
			)
		}
		for k, v := range r.Parameters {
			fmt.Fprintf(w, "    %s: %s\n", k, v)
		}
	}

	return nil
}

func writeRequirementsHuman(
	w io.Writer,
	catalogName string,
	results []requirement.AssessmentRequirementInfo,
) error {
	// TODO: add styled output with lipgloss formatting.
	return writeRequirementsText(w, catalogName, results)
}
