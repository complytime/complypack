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

func triageCmd() *cobra.Command {
	var (
		policyName string
		configPath string
		cacheDir   string
		format     string
		sources    []string
	)

	cmd := &cobra.Command{
		Use:   "triage",
		Short: "Classify assessment plans as automated or manual",
		Long: `Classify a policy's assessment plans based on evaluation
methods. Returns the automation split with executor details.

Output formats:
  human  Structured text with headers and indentation (default)
  text   Plain text suitable for CI/grep
  json   Structured JSON object

Examples:
  complypack triage --policy my-policy \
    --source oci://ghcr.io/org/catalog:v1
  complypack triage --policy my-policy --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedFormat, err := resolveFormat(format)
			if err != nil {
				return err
			}
			return runTriage(cmd, triageRunParams{
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
		"Policy or catalog name to triage (required)",
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

type triageRunParams struct {
	policyName string
	configPath string
	cacheDir   string
	format     string
	sources    []string
	stdout     io.Writer
}

func runTriage(
	cmd *cobra.Command,
	params triageRunParams,
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

	result := requirement.TriageAssessmentPlans(rp)

	switch params.format {
	case formatJSON:
		return writeTriageJSON(params.stdout, result)
	case formatText:
		return writeTriageText(params.stdout, result)
	case formatHuman:
		return writeTriageHuman(params.stdout, result)
	default:
		return fmt.Errorf(
			"unknown format %q", params.format,
		)
	}
}

func writeTriageJSON(
	w io.Writer,
	result *requirement.TriageResult,
) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func writeTriageText(
	w io.Writer,
	result *requirement.TriageResult,
) error {
	fmt.Fprintf(w, "Triage: %s\n", result.PolicyID)
	fmt.Fprintf(
		w, "  Automated: %d  Manual: %d  Total: %d\n",
		result.Counts.Automated,
		result.Counts.Manual,
		result.Counts.Total,
	)

	if len(result.Automated) > 0 {
		fmt.Fprintln(w, "\n  Automated Plans:")
		for _, p := range result.Automated {
			fmt.Fprintf(
				w, "    %s -> %s (method: %s)\n",
				p.PlanID, p.RequirementID, p.EvaluationMethod,
			)
		}
	}

	if len(result.Manual) > 0 {
		fmt.Fprintln(w, "\n  Manual Plans:")
		for _, p := range result.Manual {
			fmt.Fprintf(
				w, "    %s -> %s\n",
				p.PlanID, p.RequirementID,
			)
		}
	}

	return nil
}

func writeTriageHuman(
	w io.Writer,
	result *requirement.TriageResult,
) error {
	// TODO: add styled output with lipgloss formatting.
	return writeTriageText(w, result)
}
