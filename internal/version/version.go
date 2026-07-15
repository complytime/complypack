// SPDX-License-Identifier: Apache-2.0

// Package version exposes the module version from Go build info.
package version

import "runtime/debug"

// ModuleVersion returns the module version from Go build info, falling
// back to "(devel)" when the binary is built without version metadata.
func ModuleVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "(devel)"
}
