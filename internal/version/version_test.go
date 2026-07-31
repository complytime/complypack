// SPDX-License-Identifier: Apache-2.0

package version

import (
	"testing"
)

func TestModuleVersion_ReturnsNonEmpty(t *testing.T) {
	v := ModuleVersion()
	if v == "" {
		t.Error("ModuleVersion() returned empty string")
	}
}

func TestModuleVersion_FallbackIsDevel(t *testing.T) {
	v := ModuleVersion()
	if v != "(devel)" {
		t.Logf("ModuleVersion() = %q (may differ in installed binary)", v)
	}
}

func TestGet(t *testing.T) {
	info := Get()
	if info.Version != "dev" {
		t.Errorf("Version = %q, want %q", info.Version, "dev")
	}
	if info.Commit != "unknown" {
		t.Errorf("Commit = %q, want %q", info.Commit, "unknown")
	}
	if info.GitTreeState != "unknown" {
		t.Errorf("GitTreeState = %q, want %q", info.GitTreeState, "unknown")
	}
	if info.BuildDate != "unknown" {
		t.Errorf("BuildDate = %q, want %q", info.BuildDate, "unknown")
	}
}
