// SPDX-License-Identifier: Apache-2.0

// Package version exposes the module version from Go build info
// and structured version information set via ldflags.
package version

import "runtime/debug"

var (
	version      = "dev"
	commit       = "unknown"
	gitTreeState = "unknown"
	buildDate    = "unknown"
)

// Info holds structured version information injected at build time.
type Info struct {
	Version      string `json:"version"`
	Commit       string `json:"commit"`
	GitTreeState string `json:"gitTreeState"`
	BuildDate    string `json:"buildDate"`
}

// Get returns the structured version information.
func Get() Info {
	return Info{
		Version:      version,
		Commit:       commit,
		GitTreeState: gitTreeState,
		BuildDate:    buildDate,
	}
}

// ModuleVersion returns the module version from Go build info, falling
// back to "(devel)" when the binary is built without version metadata.
func ModuleVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "(devel)"
}
