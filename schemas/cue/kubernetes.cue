// SPDX-License-Identifier: Apache-2.0

// Platform input schema for Kubernetes resources.
// Mirrors the Kubernetes API resource object shape.
package schemas

// Common resource fields (Deployment, Pod, Service, etc.)
apiVersion?: string
kind?:       string
metadata?:   #Metadata
spec?:       #Spec

#Metadata: {
	name?:        string
	namespace?:   string
	labels?:      [string]: string
	annotations?: [string]: string
}

// Union of Deployment, Pod, and Service spec fields; all optional.
#Spec: {
	// Deployment
	replicas?: int
	template?: {
		spec?: #PodSpec
	}

	// Pod and Deployment pod template
	containers?:      [...#Container]
	securityContext?: #SecurityContext

	// Service
	type?:     string
	ports?:    [...#ServicePort]
	selector?: [string]: string
}

#PodSpec: {
	containers?:      [...#Container]
	securityContext?: #SecurityContext
}

#Container: {
	name?:            string
	image?:           string
	ports?:           [...#ContainerPort]
	resources?:       #Resources
	securityContext?: #SecurityContext
}

#ContainerPort: {
	name?:          string
	containerPort?: int
	protocol?:      string
}

#Resources: {
	limits?:   [string]: _
	requests?: [string]: _
}

#SecurityContext: {
	privileged?:      bool
	runAsNonRoot?:    bool
	runAsUser?:       int
	appArmorProfile?: _
	capabilities?: {
		drop?: [...string]
		add?:  [...string]
	}
}

#ServicePort: {
	name?:       string
	port?:       int
	targetPort?: int | string
	protocol?:   string
}

// Resource-kind aliases for documentation and policy authoring
#Deployment: {
	apiVersion?: string
	kind?:       string
	metadata?:   #Metadata
	spec?: {
		replicas?: int
		template?: {
			spec?: #PodSpec
		}
	}
}

#Pod: {
	apiVersion?: string
	kind?:       string
	metadata?:   #Metadata
	spec?:       #PodSpec
}

#Service: {
	apiVersion?: string
	kind?:       string
	metadata?:   #Metadata
	spec?: {
		type?:     string
		ports?:    [...#ServicePort]
		selector?: [string]: string
	}
}
