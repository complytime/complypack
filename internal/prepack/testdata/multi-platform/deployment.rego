package prepack.multi_platform_deployment

import rego.v1

deny contains msg if {
	input.kind == "Deployment"
	not input.spec.replicas
	msg := "Deployments must specify replicas"
}
