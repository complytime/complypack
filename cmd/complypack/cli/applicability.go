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

func applicabilityCmd() *cobra.Command {
	var (
		catalogName    string
		requirementIDs []string
		configPath     string
		cacheDir       string
		format         string
		sources        []string
	)

	cmd := &cobra.Command{
		Use:   "applicability",
		Short: "Inspect applicability groups and their requirements",
		Long: `Get applicability group definitions and their requirement
memberships from a catalog or policy. Returns group metadata
and which requirements belong to each group.

Output formats:
  human  Structured text with headers and indentation (default)
  text   Plain text suitable for CI/grep
  json   Structured JSON object

Examples:
  complypack applicability --catalog my-catalog \
    --source oci://ghcr.io/org/catalog:v1
  complypack applicability --catalog my-catalog \
    --requirement REQ-001 --requirement REQ-002
  complypack applicability --catalog my-catalog \
    --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedFormat, err := resolveFormat(format)
			if err != nil {
				return err
			}
			return runApplicability(cmd, applicabilityRunParams{
				catalogName:    catalogName,
				requirementIDs: requirementIDs,
				configPath:     configPath,
				cacheDir:       cacheDir,
				format:         resolvedFormat,
				sources:        sources,
				stdout:         cmd.OutOrStdout(),
			})
		},
	}

	cmd.Flags().StringVar(
		&catalogName, "catalog", "",
		"Catalog or policy name to inspect (required)",
	)
	cmd.Flags().StringArrayVar(
		&requirementIDs, "requirement", nil,
		"Filter to groups containing this requirement (repeatable)",
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

type applicabilityRunParams struct {
	catalogName    string
	requirementIDs []string
	configPath     string
	cacheDir       string
	format         string
	sources        []string
	stdout         io.Writer
}

func runApplicability(
	cmd *cobra.Command,
	params applicabilityRunParams,
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

	result := requirement.CollectApplicabilityGroups(
		rp, params.requirementIDs,
	)

	switch params.format {
	case formatJSON:
		return writeApplicabilityJSON(params.stdout, result)
	case formatText:
		return writeApplicabilityText(
			params.stdout, params.catalogName, result,
		)
	case formatHuman:
		return writeApplicabilityHuman(
			params.stdout, params.catalogName, result,
		)
	default:
		return fmt.Errorf(
			"unknown format %q", params.format,
		)
	}
}

func writeApplicabilityJSON(
	w io.Writer,
	result *requirement.ApplicabilityGroupResult,
) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func writeApplicabilityText(
	w io.Writer,
	catalogName string,
	result *requirement.ApplicabilityGroupResult,
) error {
	fmt.Fprintf(w, "Applicability: %s\n", catalogName)
	fmt.Fprintf(
		w, "  Groups: %d  Ungrouped: %d\n",
		len(result.Groups), len(result.Ungrouped),
	)

	for _, g := range result.Groups {
		fmt.Fprintf(w, "\n  %s", g.ID)
		if g.Title != "" {
			fmt.Fprintf(w, " - %s", g.Title)
		}
		fmt.Fprintln(w)
		if g.Description != "" {
			fmt.Fprintf(w, "    %s\n", g.Description)
		}
		if len(g.RequirementIDs) > 0 {
			fmt.Fprintf(
				w, "    Requirements: %s\n",
				strings.Join(g.RequirementIDs, ", "),
			)
		}
	}

	if len(result.Ungrouped) > 0 {
		fmt.Fprintf(
			w, "\n  Ungrouped: %s\n",
			strings.Join(result.Ungrouped, ", "),
		)
	}

	return nil
}

func writeApplicabilityHuman(
	w io.Writer,
	catalogName string,
	result *requirement.ApplicabilityGroupResult,
) error {
	// TODO: add styled output with lipgloss formatting.
	return writeApplicabilityText(w, catalogName, result)
}
