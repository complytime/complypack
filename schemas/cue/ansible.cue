// SPDX-License-Identifier: Apache-2.0

// Platform input schema for Ansible playbooks and task lists.
// Mirrors ansible-playbook YAML structure.
package schemas

hosts?:       string | [...string]
become?:      bool
become_user?: string
tasks?:       [...#Task]
handlers?:    [...#Handler]
vars?:        [string]: _
roles?:       [...string] | [...#Role]

#Task: {
	name?:        string
	module?:      string
	args?:        _
	when?:        string
	register?:    string
	notify?:      string | [...string]
	become?:      bool
	become_user?: string

	// Module FQCN or short name may appear as a task key
	[string]: _
}

#Handler: {
	name?:     string
	listen?:   string | [...string]
	module?:   string
	args?:     _
	notify?:   string | [...string]
	[string]: _
}

#Role: {
	name?:    string
	role?:    string
	vars?:    [string]: _
	tasks?:   [...#Task]
}
