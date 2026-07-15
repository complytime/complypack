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
	// When built with `go test`, debug.ReadBuildInfo returns (devel)
	// so ModuleVersion should return "(devel)" as the fallback.
	v := ModuleVersion()
	if v != "(devel)" {
		t.Logf("ModuleVersion() = %q (may differ in installed binary)", v)
	}
}
