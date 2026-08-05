// SPDX-License-Identifier: Apache-2.0

// Package testresult defines shared types for per-test outcome data,
// consumed by the tester, evaluator, and coverage packages.
package testresult

// Detail contains per-test outcome information.
type Detail struct {
	Name     string // Test rule name (e.g., "test_deny_root_user")
	Package  string // Rego package path (e.g., "data.policy.container_security")
	Location string // Source location (file:row:col)
	Passed   bool   // Whether the test passed
	Error    string // Error message if failed (empty if passed)
}
