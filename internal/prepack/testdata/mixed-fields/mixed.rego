package main

import rego.v1

deny contains msg if {
	input.spec.replicas > 3
	input.on.push.branches
	msg := "This mixes k8s and CI fields"
}
