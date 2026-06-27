package main

import rego.v1

deny contains msg if {
	input.name == ""
	msg := "Workflow must have a name"
}

deny contains msg if {
	job := input.jobs[_]
	not job.steps
	msg := "Jobs must have steps"
}
