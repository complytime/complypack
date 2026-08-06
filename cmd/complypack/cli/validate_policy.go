// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/internal/schema"
	"github.com/spf13/cobra"
)

// validatePolicyRunParams holds parsed CLI parameters for the validate-policy command.
type validatePolicyRunParams struct {
	policyFile string
	platform   string
	evalID     string
	schemas    []string
	format     string
	stdout     io.Writer
}

// validatePolicyResult holds the aggregated validation results.
type validatePolicyResult struct {
	valid              bool
	syntaxErrors       []string
	contractViolations []map[string]string
	lintWarnings       []map[string]string
}

func validatePolicyCmd() *cobra.Command {
	var (
		platform string
		evalID   string
		schemas  []string
		format   string
	)

	cmd := &cobra.Command{
		Use:   "validate-policy <file>",
		Short: "Validate a policy file for syntax, contract compliance, and lint",
		Long: `Validate a policy file against a platform schema.

Performs three checks in sequence:
  1. Syntax validation — is the policy syntactically valid?
  2. Contract validation — do all input.* references exist in the platform schema?
  3. Lint — does the policy follow style and quality rules?

Contract and lint checks are skipped when syntax errors exist.
Lint failures are non-fatal and do not affect the valid/invalid verdict.

Output formats:
  human  Styled output with Unicode symbols and color (default for terminals)
  text   Plain bracketed labels for CI/grep
  json   Structured JSON matching the MCP validate_policy tool response

Examples:
  complypack validate-policy policy.rego --platform kubernetes-deployment
  complypack validate-policy policy.rego --platform kubernetes-deployment --format json
  complypack validate-policy policy.rego --platform ci-github-actions \
    --schema ci-github-actions=cue://example.com/schema`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedFormat, err := resolveFormat(format)
			if err != nil {
				return err
			}
			return runValidatePolicy(cmd.Context(), validatePolicyRunParams{
				policyFile: args[0],
				platform:   platform,
				evalID:     evalID,
				schemas:    schemas,
				format:     resolvedFormat,
				stdout:     cmd.OutOrStdout(),
			})
		},
	}

	cmd.Flags().StringVar(&platform, "platform", "",
		"Target platform for contract validation (required)")
	cmd.Flags().StringVar(&evalID, "evaluator", "",
		"Evaluator ID (auto-detected if omitted)")
	cmd.Flags().StringArrayVar(&schemas, "schema", nil,
		"Platform schema override (repeatable, e.g., platform=source)")
	cmd.Flags().StringVarP(&format, "format", "f", "",
		"Output format: human, text, or json (default: auto-detected)")

	_ = cmd.MarkFlagRequired("platform")

	_ = cmd.RegisterFlagCompletionFunc("format",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return []string{formatHuman, formatText, formatJSON},
				cobra.ShellCompDirectiveNoFileComp
		})

	return cmd
}

func runValidatePolicy(
	ctx context.Context,
	params validatePolicyRunParams,
) error {
	// Read policy file
	content, err := os.ReadFile(params.policyFile)
	if err != nil {
		return fmt.Errorf("reading policy file: %w", err)
	}

	// Resolve evaluator
	evalRegistry := evaluator.DefaultRegistry()
	var eval evaluator.Evaluator
	if params.evalID != "" {
		eval, err = evalRegistry.Get(params.evalID)
		if err != nil {
			return fmt.Errorf("evaluator %q: %w", params.evalID, err)
		}
	} else {
		ids := evalRegistry.IDs()
		if len(ids) == 0 {
			return fmt.Errorf("no evaluators registered")
		}
		if len(ids) > 1 {
			return fmt.Errorf(
				"multiple evaluators available (%s); use --evaluator to select one",
				strings.Join(ids, ", "),
			)
		}
		eval, _ = evalRegistry.Get(ids[0])
	}

	filename := "policy" + eval.FileExtension()
	policyContent := string(content)

	// Syntax validation
	syntaxErrs := eval.Validate(filename, policyContent)

	result := validatePolicyResult{
		syntaxErrors:       make([]string, 0),
		contractViolations: make([]map[string]string, 0),
		lintWarnings:       make([]map[string]string, 0),
	}

	for _, e := range syntaxErrs {
		result.syntaxErrors = append(result.syntaxErrors, e.Error())
	}

	// Contract and lint checks (only when syntax is valid)
	if len(syntaxErrs) == 0 {
		// Load CUE schema for the platform
		schemaRefs, err := buildSchemaRefs(params.platform, params.schemas)
		if err != nil {
			return err
		}

		_, cueSchemas, err := schema.LoadFromConfig(ctx, schemaRefs, schema.DefaultRegistry())
		if err != nil {
			return fmt.Errorf("loading schemas: %w", err)
		}

		cueSchema, ok := cueSchemas[params.platform]
		if !ok {
			return fmt.Errorf(
				"no schema found for platform %q; provide --schema %s=<source>",
				params.platform, params.platform,
			)
		}

		// Contract validation
		violations, err := eval.CheckContract(filename, policyContent, cueSchema)
		if err != nil {
			return fmt.Errorf("contract check failed: %w", err)
		}
		for _, v := range violations {
			result.contractViolations = append(result.contractViolations, map[string]string{
				"path":     v.Path,
				"location": v.Location,
			})
		}

		// Lint (graceful degradation)
		lintWarnings, _ := eval.Lint(filename, policyContent)
		for _, w := range lintWarnings {
			result.lintWarnings = append(result.lintWarnings, map[string]string{
				"rule":     w.Rule,
				"message":  w.Message,
				"location": w.Location,
			})
		}
	}

	result.valid = len(result.syntaxErrors) == 0 && len(result.contractViolations) == 0

	return writeValidatePolicyOutput(params.format, params.stdout, result)
}

func writeValidatePolicyOutput(
	format string, w io.Writer, result validatePolicyResult,
) error {
	switch format {
	case formatJSON:
		return writeValidatePolicyJSON(w, result)
	case formatText:
		return writeValidatePolicyText(w, result)
	case formatHuman:
		return writeValidatePolicyHuman(w, result)
	default:
		return fmt.Errorf(
			"unknown format %q; valid formats: human, text, json",
			format,
		)
	}
}

func writeValidatePolicyJSON(w io.Writer, result validatePolicyResult) error {
	response := map[string]interface{}{
		"valid":              result.valid,
		"syntaxErrors":       result.syntaxErrors,
		"contractViolations": result.contractViolations,
		"lintWarnings":       result.lintWarnings,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(response)
}

func writeValidatePolicyText(w io.Writer, result validatePolicyResult) error {
	if result.valid {
		fmt.Fprintln(w, "[VALID] Policy is valid")
	} else {
		fmt.Fprintln(w, "[INVALID] Policy has errors")
	}

	for _, e := range result.syntaxErrors {
		fmt.Fprintf(w, "  [SYNTAX ERROR] %s\n", e)
	}
	for _, v := range result.contractViolations {
		fmt.Fprintf(w, "  [CONTRACT VIOLATION] %s at %s\n", v["path"], v["location"])
	}
	for _, lw := range result.lintWarnings {
		fmt.Fprintf(w, "  [LINT WARNING] %s: %s at %s\n",
			lw["rule"], lw["message"], lw["location"])
	}

	return nil
}

func writeValidatePolicyHuman(w io.Writer, result validatePolicyResult) error {
	if result.valid {
		fmt.Fprintln(w, stylePass.Render("✓ Policy is valid"))
	} else {
		fmt.Fprintln(w, styleFail.Render("✗ Policy is invalid"))
	}

	if len(result.syntaxErrors) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, styleTitle.Render("Syntax Errors"))
		for _, e := range result.syntaxErrors {
			fmt.Fprintf(w, "  %s %s\n", styleFail.Render("✗"), e)
		}
	}

	if len(result.contractViolations) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, styleTitle.Render("Contract Violations"))
		for _, v := range result.contractViolations {
			fmt.Fprintf(w, "  %s %s at %s\n",
				styleFail.Render("✗"), v["path"], v["location"])
		}
	}

	if len(result.lintWarnings) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, styleTitle.Render("Lint Warnings"))
		for _, lw := range result.lintWarnings {
			fmt.Fprintf(w, "  %s %s: %s at %s\n",
				styleWarn.Render("⚠"), lw["rule"], lw["message"], lw["location"])
		}
	}

	return nil
}
