// SPDX-License-Identifier: Apache-2.0

package tester

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
)

// FixtureResults contains fixture-based test results.
type FixtureResults struct {
	Total  int            // Total number of fixtures tested
	Passed int            // Number of passing fixtures
	Failed int            // Number of failing fixtures
	Errors []FixtureError // Error details for failing fixtures
}

// FixtureError describes a fixture test failure.
type FixtureError struct {
	Fixture    string   // Fixture filename
	Expected   string   // Expected outcome (allow/deny)
	Violations []string // Policy violations found (empty if expected allow but got deny)
}

// RunFixtures evaluates policies against JSON fixture inputs.
// Naming conventions:
//   - Filenames with "_allow", "_valid", or starting with "valid" expect no violations
//   - Filenames with "_deny", "_invalid", or starting with "invalid" expect violations
//
// fixtureDir: directory containing JSON fixture files (can have platform subdirs)
// policyDir: directory containing Rego policies (can have platform subdirs)
func RunFixtures(ctx context.Context, fixtureDir string, policyDir string) (*FixtureResults, error) {
	// Load policies
	policies, err := loadPolicies(policyDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load policies: %w", err)
	}

	if len(policies) == 0 {
		return &FixtureResults{}, nil
	}

	// Discover deny rules
	denyRules, err := discoverDenyRules(policies)
	if err != nil {
		return nil, fmt.Errorf("failed to discover deny rules: %w", err)
	}

	if len(denyRules) == 0 {
		return nil, fmt.Errorf("no deny rules found in policies")
	}

	// Load fixtures
	fixtures, err := loadFixtures(fixtureDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load fixtures: %w", err)
	}

	if len(fixtures) == 0 {
		return &FixtureResults{}, nil
	}

	// Prepare Rego query
	query, err := prepareQuery(ctx, policies, denyRules)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query: %w", err)
	}

	// Evaluate each fixture
	results := &FixtureResults{}
	for name, input := range fixtures {
		results.Total++

		expected := expectation(name)
		violations, err := evaluateFixture(ctx, query, input)
		if err != nil {
			results.Failed++
			results.Errors = append(results.Errors, FixtureError{
				Fixture:    name,
				Expected:   expected,
				Violations: []string{err.Error()},
			})
			continue
		}

		// Check if result matches expectation
		if expected == "allow" && len(violations) == 0 {
			results.Passed++
		} else if expected == "deny" && len(violations) > 0 {
			results.Passed++
		} else {
			results.Failed++
			results.Errors = append(results.Errors, FixtureError{
				Fixture:    name,
				Expected:   expected,
				Violations: violations,
			})
		}
	}

	return results, nil
}

// loadPolicies reads all .rego files from the policy directory.
func loadPolicies(dir string) (map[string]string, error) {
	policies := make(map[string]string)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".rego") {
			content, err := os.ReadFile(path) //nolint:gosec // G122: path is validated by filepath.Walk
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", path, err)
			}
			policies[path] = string(content)
		}
		return nil
	})

	return policies, err
}

// discoverDenyRules walks the AST to find all deny rule names with their package paths.
func discoverDenyRules(policies map[string]string) ([]string, error) {
	var rules []string
	seen := make(map[string]bool)

	for filename, src := range policies {
		mod, err := ast.ParseModuleWithOpts(filename, src, ast.ParserOptions{RegoVersion: ast.RegoV1})
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
		}

		// Get package path
		pkg := mod.Package.Path.String()

		for _, rule := range mod.Rules {
			name := string(rule.Head.Name)
			if strings.HasPrefix(name, "deny") {
				// Build full reference: data.package.rule
				fullRef := fmt.Sprintf("%s.%s", pkg, name)
				if !seen[fullRef] {
					rules = append(rules, fullRef)
					seen[fullRef] = true
				}
			}
		}
	}

	return rules, nil
}

// loadFixtures reads all .json files from the fixture directory.
func loadFixtures(dir string) (map[string]interface{}, error) {
	fixtures := make(map[string]interface{})

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".json") {
			content, err := os.ReadFile(path) //nolint:gosec // G122: path is validated by filepath.Walk
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", path, err)
			}

			var data interface{}
			if err := json.Unmarshal(content, &data); err != nil {
				return fmt.Errorf("failed to parse JSON %s: %w", path, err)
			}

			// Use relative path as fixture name
			name := strings.TrimPrefix(path, dir+"/")
			fixtures[name] = data
		}
		return nil
	})

	return fixtures, err
}

// expectation determines expected outcome from fixture filename.
func expectation(name string) string {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "_allow") || strings.Contains(lower, "_valid") || strings.HasPrefix(lower, "valid") {
		return "allow"
	}
	return "deny"
}

// prepareQuery creates a Rego prepared query for all deny rules.
func prepareQuery(ctx context.Context, policies map[string]string, denyRules []string) (rego.PreparedEvalQuery, error) {
	// Build query string for all deny rules (they already have data. prefix)
	query := strings.Join(denyRules, "; ")

	// Compile policies
	modules := make([]func(*rego.Rego), 0, len(policies))
	for filename, src := range policies {
		filename := filename
		src := src
		modules = append(modules, rego.Module(filename, src))
	}

	// Prepare query
	r := rego.New(
		append(modules,
			rego.Query(query),
		)...,
	)

	return r.PrepareForEval(ctx)
}

// evaluateFixture runs the query against a fixture and returns violations.
func evaluateFixture(ctx context.Context, query rego.PreparedEvalQuery, input interface{}) ([]string, error) {
	rs, err := query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return nil, fmt.Errorf("evaluation failed: %w", err)
	}

	var violations []string
	for _, result := range rs {
		for _, expr := range result.Expressions {
			// Each expression value could be a set of violations
			switch v := expr.Value.(type) {
			case []interface{}:
				for _, item := range v {
					if str, ok := item.(string); ok && str != "" {
						violations = append(violations, str)
					}
				}
			case string:
				if v != "" {
					violations = append(violations, v)
				}
			}
		}
	}

	return violations, nil
}
