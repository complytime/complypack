package prepack.contract_violation

import rego.v1

deny contains msg if {
	input.kind == "Pod"
	not input.metadata.invalid_field
	msg := "Contract violation example"
}
