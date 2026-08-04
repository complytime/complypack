// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/complytime/complypack/internal/cache"
	"github.com/complytime/complypack/internal/requirement"
	"github.com/spf13/cobra"
)

func deltaCmd() *cobra.Command {
	var (
		policyName string
		configPath string
		cacheDir   string
		format     string
		sources    []string
	)

	cmd := &cobra.Command{
		Use:   "delta",
		Short: "Analyze parameter differences across catalog layers",
		Long: `Gather parameter comparisons across a resolved policy.
Returns structured L3 parameter values alongside the L1/L2
requirement text they map to.

Output formats:
  human  Structured text with headers and indentation (default)
  text   Plain text suitable for CI/grep
  json   Structured JSON object

Examples:
  complypack delta --policy my-policy \
    --source oci://ghcr.io/org/catalog:v1
  complypack delta --policy my-policy --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedFormat, err := resolveFormat(format)
			if err != nil {
				return err
			}
			return runDelta(cmd, deltaRunParams{
				policyName: policyName,
				configPath: configPath,
				cacheDir:   cacheDir,
				format:     resolvedFormat,
				sources:    sources,
				stdout:     cmd.OutOrStdout(),
			})
		},
	}

	cmd.Flags().StringVar(
		&policyName, "policy", "",
		"Policy or catalog name to analyze (required)",
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

	_ = cmd.MarkFlagRequired("policy")

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

type deltaRunParams struct {
	policyName string
	configPath string
	cacheDir   string
	format     string
	sources    []string
	stdout     io.Writer
}

func runDelta(
	cmd *cobra.Command,
	params deltaRunParams,
) error {
	ctx := cmd.Context()

	loadResult, err := loadArtifacts(
		ctx, params.sources, params.configPath, params.cacheDir,
	)
	if err != nil {
		return err
	}

	rp, err := findResolvedPolicyCLI(
		loadResult, params.policyName,
	)
	if err != nil {
		return err
	}

	report, err := requirement.AnalyzeDelta(
		rp, loadResult.Artifacts,
	)
	if err != nil {
		return fmt.Errorf("delta analysis failed: %w", err)
	}

	switch params.format {
	case formatJSON:
		return writeDeltaJSON(params.stdout, report)
	case formatText:
		return writeDeltaText(params.stdout, report)
	case formatHuman:
		return writeDeltaHuman(params.stdout, report)
	default:
		return fmt.Errorf(
			"unknown format %q", params.format,
		)
	}
}

func writeDeltaJSON(
	w io.Writer,
	report *requirement.DeltaReport,
) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func writeDeltaText(
	w io.Writer,
	report *requirement.DeltaReport,
) error {
	fmt.Fprintf(w, "Delta: %s\n", report.PolicyID)
	fmt.Fprintf(
		w, "  Catalogs compared: %d\n",
		len(report.CatalogsCompared),
	)
	fmt.Fprintf(
		w, "  Comparisons: %d\n",
		len(report.Comparisons),
	)

	for _, c := range report.Comparisons {
		fmt.Fprintf(
			w, "\n  %s / %s\n", c.RequirementID, c.Label,
		)
		fmt.Fprintf(w, "    Policy value: %s\n", c.PolicyValue)
		if c.RequirementText != "" {
			fmt.Fprintf(
				w, "    Requirement: %s\n", c.RequirementText,
			)
		}
	}

	return nil
}

func writeDeltaHuman(
	w io.Writer,
	report *requirement.DeltaReport,
) error {
	// TODO: add styled output with lipgloss formatting.
	return writeDeltaText(w, report)
}
