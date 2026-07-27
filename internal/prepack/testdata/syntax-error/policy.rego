package prepack.syntax_error

import rego.v1

deny contains msg if {
	input.kind == "Pod"
	not input.metadata.labels["app"]
	msg := "Pods must have 'app' label"
  # Missing closing brace
