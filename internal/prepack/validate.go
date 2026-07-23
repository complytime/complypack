// SPDX-License-Identifier: Apache-2.0

package prepack

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"github.com/complytime/complypack/internal/evaluator"
)

// ValidationOptions controls which stages run in the pre-pack pipeline.
type ValidationOptions struct {
	SkipTests bool
}

// ValidationResult holds the outcome of the pre-pack validation pipeline.
type ValidationResult struct {
	FilesChecked       int
	MissingFiles       []string
	SyntaxErrors       []FileError
	ContractViolations []evaluator.ContractViolation
	TestResults        *evaluator.TestResults
	TestsSkipped       bool
}

// FileError associates a validation error with the file that caused it.
type FileError struct {
	File  string
	Error string
}

// Valid returns true if there are no syntax errors, contract violations,
// or test failures.
func (r *ValidationResult) Valid() bool {
	if len(r.MissingFiles) > 0 {
		return false
	}
	if len(r.SyntaxErrors) > 0 || len(r.ContractViolations) > 0 {
		return false
	}
	if r.TestResults != nil && r.TestResults.Failed > 0 {
		return false
	}
	return true
}

// Validate runs the pre-pack validation pipeline against a content directory.
// Four stages execute in order, each fail-fast:
//  0. Required files -- verify evaluator-specific companion files exist
//  1. Syntax check -- parse non-test policy files individually
//  2. Contract check -- verify input.* references against CUE schema
//  3. Test execution -- run policy unit tests
//
// If cueSchemas is empty, contract checking is skipped with a warning.
func Validate(ctx context.Context, contentDir string, eval evaluator.Evaluator, cueSchemas []cue.Value, opts ValidationOptions) (*ValidationResult, error) {
	result := &ValidationResult{}

	ext := eval.FileExtension()
	// matchedFiles includes both policy and test files; test files are
	// filtered downstream in each stage that requires it.
	matchedFiles, err := collectFiles(contentDir, ext)
	if err != nil {
		return nil, fmt.Errorf("collecting policy files: %w", err)
	}

	// Stage 0: Required files check — runs after directory access
	// is confirmed but before policy file processing.
	for _, reqFile := range eval.RequiredFiles() {
		reqPath := filepath.Join(contentDir, reqFile)
		if _, statErr := os.Stat(reqPath); errors.Is(statErr, os.ErrNotExist) {
			result.MissingFiles = append(result.MissingFiles, reqFile)
		} else if statErr != nil {
			return nil, fmt.Errorf("checking required file %s: %w", reqFile, statErr)
		}
	}
	if len(result.MissingFiles) > 0 {
		return result, nil
	}

	if len(matchedFiles) == 0 {
		slog.Warn("no policy files found", "dir", contentDir, "extension", ext)
		return result, nil
	}

	// Stage 1: Syntax check — read all files but only syntax-check
	// non-test files.  Test files reference rules from companion
	// policy files and cannot be compiled in isolation (single-module
	// compilation would produce rego_unsafe_var_error).  They are
	// validated jointly in Stage 3 (test execution) instead.
	var policyCount int
	fileSources := make(map[string]string, len(matchedFiles))
	for _, path := range matchedFiles {
		data, readErr := os.ReadFile(path) //nolint:gosec // G304 -- path from controlled WalkDir
		if readErr != nil {
			return nil, fmt.Errorf("reading %s: %w", path, readErr)
		}
		src := string(data)

		relPath, _ := filepath.Rel(contentDir, path)
		fileSources[relPath] = src

		if IsTestFile(relPath, ext) {
			continue
		}
		policyCount++

		syntaxErrs := eval.Validate(relPath, src)
		for _, e := range syntaxErrs {
			result.SyntaxErrors = append(result.SyntaxErrors, FileError{
				File:  relPath,
				Error: e.Error(),
			})
		}
	}

	result.FilesChecked = policyCount
	slog.Info("validating policies", "files", result.FilesChecked, "evaluator", eval.ID())

	if len(result.SyntaxErrors) > 0 {
		return result, nil
	}

	// Stage 2: Contract check — pass if ANY schema validates
	if len(cueSchemas) == 0 {
		slog.Warn("no CUE schemas provided, skipping contract validation")
	} else {
		for relPath, src := range fileSources {
			if IsTestFile(relPath, ext) {
				continue
			}
			var bestViolations []evaluator.ContractViolation
			passed := false
			for _, s := range cueSchemas {
				if !s.Exists() {
					continue
				}
				violations, contractErr := eval.CheckContract(relPath, src, s)
				if contractErr != nil {
					return nil, fmt.Errorf("contract check failed for %s: %w", relPath, contractErr)
				}
				if len(violations) == 0 {
					passed = true
					break
				}
				if bestViolations == nil || len(violations) < len(bestViolations) {
					bestViolations = violations
				}
			}
			if !passed && bestViolations != nil {
				result.ContractViolations = append(result.ContractViolations, bestViolations...)
			}
		}

		if len(result.ContractViolations) > 0 {
			return result, nil
		}
	}

	// Stage 3: Test execution
	if opts.SkipTests {
		result.TestsSkipped = true
		return result, nil
	}

	testFiles := filterTestFiles(fileSources, ext)
	if len(testFiles) == 0 {
		slog.Info("no test files found, skipping test execution")
		result.TestsSkipped = true
		return result, nil
	}

	// Include both policy and test files for execution
	testResults, testErr := eval.Test(ctx, fileSources)
	if testErr != nil {
		return nil, fmt.Errorf("test execution failed: %w", testErr)
	}
	result.TestResults = testResults

	return result, nil
}

// collectFiles walks dir and returns paths matching the given extension.
// Skips hidden files/directories.
func collectFiles(dir string, ext string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ext) {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// IsTestFile returns true if the filename follows the *_test.<ext> convention.
func IsTestFile(name string, ext string) bool {
	return strings.HasSuffix(name, "_test"+ext)
}

// filterTestFiles returns only files matching the *_test.<ext> pattern.
func filterTestFiles(files map[string]string, ext string) map[string]string {
	result := make(map[string]string)
	for name, src := range files {
		if IsTestFile(name, ext) {
			result[name] = src
		}
	}
	return result
}
