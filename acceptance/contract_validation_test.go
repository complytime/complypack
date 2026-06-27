// SPDX-License-Identifier: Apache-2.0

package acceptance_test

import (
	"context"

	"github.com/complytime/complypack/internal/schema"
	"github.com/complytime/complypack/internal/validator"
	"github.com/complytime/complypack/schemas"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Contract Validation", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("CUE registry module with definition fragment", func() {
		It("validates GitHub Actions workflow paths without false positives", func() {
			reg := schema.DefaultRegistry()
			s, err := reg.Load(ctx, "cue://cue.dev/x/githubactions@v0#Workflow", "ci-github-actions")
			Expect(err).NotTo(HaveOccurred())

			policy := `package ci.example
import rego.v1

deny contains msg if {
    input.name == ""
    msg := "workflow must have a name"
}

deny contains msg if {
    job := input.jobs[_]
    msg := "test"
}

deny contains msg if {
    input.on.push.branches
    msg := "test"
}
`
			violations, err := validator.CheckContract("policy.rego", policy, s.CUE)
			Expect(err).NotTo(HaveOccurred())
			Expect(violations).To(BeEmpty(), "valid GitHub Actions paths should not produce violations")
		})

		It("returns error when fragment is missing on definitions-only module", func() {
			reg := schema.DefaultRegistry()
			_, err := reg.Load(ctx, "cue://cue.dev/x/githubactions@v0", "ci-github-actions")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("#Workflow"))
		})
	})

	Describe("Schema index resolution", func() {
		It("loads Kubernetes Deployment schema from index and validates paths", func() {
			index, err := schemas.LoadIndex()
			Expect(err).NotTo(HaveOccurred())

			entry, ok := index["kubernetes-deployment"]
			Expect(ok).To(BeTrue())

			reg := schema.DefaultRegistry()
			s, err := reg.Load(ctx, entry.Source, "kubernetes-deployment")
			Expect(err).NotTo(HaveOccurred())

			policy := `package kubernetes.deployment
import rego.v1

deny contains msg if {
    input.metadata.name == ""
    msg := "Deployment must have a name"
}

deny contains msg if {
    not input.spec.replicas
    msg := "Deployment must specify replicas"
}
`
			violations, err := validator.CheckContract("policy.rego", policy, s.CUE)
			Expect(err).NotTo(HaveOccurred())
			Expect(violations).To(BeEmpty(), "valid Deployment paths should not produce violations")
		})

		It("rejects bogus paths against strict upstream schema", func() {
			index, err := schemas.LoadIndex()
			Expect(err).NotTo(HaveOccurred())

			entry := index["ci-github-actions"]

			reg := schema.DefaultRegistry()
			s, err := reg.Load(ctx, entry.Source, "ci-github-actions")
			Expect(err).NotTo(HaveOccurred())

			policy := `package ci.example
import rego.v1

deny contains msg if {
    input.completely_bogus
    msg := "test"
}
`
			violations, err := validator.CheckContract("policy.rego", policy, s.CUE)
			Expect(err).NotTo(HaveOccurred())
			Expect(violations).NotTo(BeEmpty(), "bogus paths should produce violations against strict schema")
		})
	})
})
