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
		t.Errorf("ModuleVersion() = %q, want %q", v, "(devel)")
	}
}

func TestGet_DefaultValues(t *testing.T) {
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

func TestGet_ReflectsInjectedValues(t *testing.T) {
	origVersion, origCommit := version, commit
	origTree, origDate := gitTreeState, buildDate
	t.Cleanup(func() {
		version, commit = origVersion, origCommit
		gitTreeState, buildDate = origTree, origDate
	})

	version = "v1.2.3"
	commit = "abc1234"
	gitTreeState = "clean"
	buildDate = "2026-07-31T00:00:00Z"

	info := Get()
	if info.Version != "v1.2.3" {
		t.Errorf("Version = %q, want %q", info.Version, "v1.2.3")
	}
	if info.Commit != "abc1234" {
		t.Errorf("Commit = %q, want %q", info.Commit, "abc1234")
	}
	if info.GitTreeState != "clean" {
		t.Errorf("GitTreeState = %q, want %q", info.GitTreeState, "clean")
	}
	if info.BuildDate != "2026-07-31T00:00:00Z" {
		t.Errorf("BuildDate = %q, want %q", info.BuildDate, "2026-07-31T00:00:00Z")
	}
}
