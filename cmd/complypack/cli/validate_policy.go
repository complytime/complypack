// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/internal/schema"
	"github.com/spf13/cobra"
)

// errPolicyInvalid is returned when validation
// completes successfully but the policy is invalid.
// It triggers a non-zero exit code.
var errPolicyInvalid = errors.New(
	"policy validation failed",
)

// validatePolicyRunParams holds parsed CLI parameters
// for the validate-policy command.
type validatePolicyRunParams struct {
	policyFile string
	platform   string
	evalID     string
	schemas    []string
	format     string
	stdout     io.Writer
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

Exit codes:
  0  Policy is valid
  1  Policy is invalid (syntax errors, contract violations, or both)

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
	eval, err := evaluator.DefaultRegistry().Resolve(
		params.evalID,
	)
	if err != nil {
		return fmt.Errorf("resolving evaluator: %w", err)
	}

	// Load CUE schema for the platform
	schemaRefs, err := buildSchemaRefs(
		params.platform, params.schemas,
	)
	if err != nil {
		return fmt.Errorf(
			"building schema refs: %w", err,
		)
	}

	_, cueSchemas, err := schema.LoadFromConfig(
		ctx, schemaRefs, schema.DefaultRegistry(),
	)
	if err != nil {
		return fmt.Errorf("loading schemas: %w", err)
	}

	cueSchema, ok := cueSchemas[params.platform]
	if !ok {
		return fmt.Errorf(
			"no schema found for platform %q;"+
				" provide --schema %s=<source>",
			params.platform, params.platform,
		)
	}

	// Delegate to domain function
	domainResult := evaluator.ValidatePolicy(
		eval, string(content), cueSchema,
	)

	// Convert to CLI presentation model
	result := convertValidatePolicyResult(domainResult)

	if err := writeValidatePolicyOutput(
		params.format, params.stdout, result,
	); err != nil {
		return err
	}

	if !result.valid {
		return errPolicyInvalid
	}
	return nil
}

// convertValidatePolicyResult maps the domain
// ValidatePolicyResult to the CLI presentation model.
func convertValidatePolicyResult(
	dr *evaluator.ValidatePolicyResult,
) validatePolicyResult {
	violations := make(
		[]map[string]string, 0, len(dr.ContractViolations),
	)
	for _, v := range dr.ContractViolations {
		violations = append(
			violations, map[string]string{
				"path":     v.Path,
				"location": v.Location,
			},
		)
	}

	warnings := make(
		[]map[string]string, 0, len(dr.LintWarnings),
	)
	for _, w := range dr.LintWarnings {
		warnings = append(
			warnings, map[string]string{
				"rule":     w.Rule,
				"message":  w.Message,
				"location": w.Location,
			},
		)
	}

	return validatePolicyResult{
		valid:              dr.Valid,
		syntaxErrors:       dr.SyntaxErrors,
		contractViolations: violations,
		lintWarnings:       warnings,
		lintErr:            dr.LintErr,
	}
}

// validatePolicyResult holds the CLI presentation model
// for validation results.
type validatePolicyResult struct {
	valid              bool
	syntaxErrors       []string
	contractViolations []map[string]string
	lintWarnings       []map[string]string
	lintErr            error
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

func writeValidatePolicyJSON(
	w io.Writer, result validatePolicyResult,
) error {
	response := map[string]interface{}{
		"valid":              result.valid,
		"syntaxErrors":       result.syntaxErrors,
		"contractViolations": result.contractViolations,
		"lintWarnings":       result.lintWarnings,
	}
	if result.lintErr != nil {
		response["lintError"] = result.lintErr.Error()
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
	if result.lintErr != nil {
		fmt.Fprintf(w,
			"  [LINT ERROR] %s\n", result.lintErr)
	}

	return nil
}

func writeValidatePolicyHuman(
	w io.Writer, result validatePolicyResult,
) error {
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
		fmt.Fprintln(w,
			styleTitle.Render("Lint Warnings"))
		for _, lw := range result.lintWarnings {
			fmt.Fprintf(w, "  %s %s: %s at %s\n",
				styleWarn.Render("⚠"),
				lw["rule"], lw["message"],
				lw["location"])
		}
	}

	if result.lintErr != nil {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %s lint: %s\n",
			styleWarn.Render("⚠"), result.lintErr)
	}

	return nil
}
