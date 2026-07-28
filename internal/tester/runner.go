// SPDX-License-Identifier: Apache-2.0

package tester

import (
	"context"
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/tester"
)

// Results contains policy test execution results.
type Results struct {
	Total  int      // Total number of tests
	Passed int      // Number of passing tests
	Failed int      // Number of failing tests
	Errors []string // Error messages from failing tests
}

// Run executes OPA policy unit tests.
// files is a map of filename -> source code.
// Returns test results or an error if tests cannot be executed.
func Run(ctx context.Context, files map[string]string) (*Results, error) {
	if len(files) == 0 {
		return &Results{}, nil
	}

	// Parse all Rego modules, skipping non-Rego files
	modules := make(map[string]*ast.Module, len(files))
	for name, src := range files {
		if !strings.HasSuffix(name, ".rego") {
			continue
		}
		mod, err := ast.ParseModuleWithOpts(name, src, ast.ParserOptions{RegoVersion: ast.RegoV1})
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", name, err)
		}
		modules[name] = mod
	}

	// Compile all modules together
	compiler := ast.NewCompiler()
	compiler.Compile(modules)
	if compiler.Failed() {
		return nil, fmt.Errorf("compilation failed: %v", compiler.Errors)
	}

	// Run tests
	runner := tester.NewRunner().SetCompiler(compiler).SetModules(modules)
	ch, err := runner.RunTests(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to run tests: %w", err)
	}

	// Collect results
	results := &Results{}
	for result := range ch {
		results.Total++

		if result.Fail || result.Error != nil {
			results.Failed++
			if result.Error != nil {
				results.Errors = append(results.Errors, fmt.Sprintf("%s: %s", result.Location, result.Error))
			} else {
				results.Errors = append(results.Errors, fmt.Sprintf("%s: test failed", result.Location))
			}
		} else if !result.Skip {
			results.Passed++
		}
	}

	return results, nil
}
