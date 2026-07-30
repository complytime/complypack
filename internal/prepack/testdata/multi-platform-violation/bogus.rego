package prepack.multi_platform_violation

import rego.v1

deny contains msg if {
	input.completely_nonexistent.deeply_nested
	msg := "This path exists in no schema"
}
