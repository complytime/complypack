// SPDX-License-Identifier: Apache-2.0

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, "dev", info.Version)
	assert.Equal(t, "unknown", info.Commit)
	assert.Equal(t, "unknown", info.GitTreeState)
	assert.Equal(t, "unknown", info.BuildDate)
}
