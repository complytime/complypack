package main

import rego.v1

deny contains msg if {
	input.kind == "Pod"
	not input.metadata.name
	msg := "Pods must have a name"
}
