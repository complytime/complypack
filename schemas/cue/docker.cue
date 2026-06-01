// SPDX-License-Identifier: Apache-2.0

// Platform input schema for Dockerfile analysis.
// Mirrors parsed Dockerfile instruction lists from build scanners.
package schemas

instructions?: [...#Instruction]
stages?:       [...#Stage]
base_image?:   string
user?:         string

#Instruction: {
	// Instruction keyword: FROM, RUN, COPY, ADD, EXPOSE, USER, ENV, etc.
	type?:  string
	value?: [...string] | string
	flags?: [...string]

	// Alternate field names used by some Dockerfile parsers
	cmd?:   string
	...
}

#Stage: {
	name?:         string
	base_image?:   string
	instructions?: [...#Instruction]
}
