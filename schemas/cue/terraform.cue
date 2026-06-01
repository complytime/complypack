// SPDX-License-Identifier: Apache-2.0

// Platform input schema for Terraform plan and configuration JSON.
// Mirrors terraform show -json and plan file structure.
package schemas

resource_changes?: [...#ResourceChange]

configuration?: {
	provider_config?: [string]: #ProviderConfig
}

planned_values?: {
	root_module?: {
		resources?: [...#PlannedResource]
	}
}

#ResourceChange: {
	type?: string
	name?: string
	change?: {
		actions?: [...string]
		after?:   #ResourceAttributes & _
		before?:  #ResourceAttributes & _
	}
}

#ProviderConfig: {
	name?:           string
	full_name?:      string
	version_constraint?: string
	expressions?: [string]: _
}

#PlannedResource: {
	address?:       string
	mode?:          string
	type?:          string
	name?:          string
	provider_name?: string
	values?:        #ResourceAttributes & _
}

// Common resource attributes across cloud providers
#ResourceAttributes: {
	tags?:         [string]: string
	region?:       string
	encryption?:   _
	encrypted?:    bool
	kms_key_id?:   string
	kms_key_name?: string
}
