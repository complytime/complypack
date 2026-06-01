// SPDX-License-Identifier: Apache-2.0

// Platform input schema for CI/CD configurations.
// Union of GitLab CI and GitHub Actions fields since helpers support both.
package schemas

// GitLab CI fields
stages?: [...string]

// GitHub Actions fields
name?: string
on?:   _
jobs?: [string]: #Job

// GitLab allows arbitrary job names as top-level keys
[string]: #Job | [...string] | _

#Job: {
	// GitHub Actions
	"runs-on"?: string
	steps?: [...#Step]

	// GitLab CI
	stage?:  string
	script?: [...string]
	image?:  string
	...
}

#Step: {
	uses?: string
	run?:  string
	name?: string
	with?: [string]: string
	env?: [string]:  string
	...
}
