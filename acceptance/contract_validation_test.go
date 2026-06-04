// SPDX-License-Identifier: Apache-2.0

package acceptance_test

import (
	"context"

	"cuelang.org/go/cue/cuecontext"
	"github.com/complytime/complypack/internal/mcp"
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
			source, err := mcp.ParseSchemaSource("cue://cue.dev/x/githubactions@v0#Workflow")
			Expect(err).NotTo(HaveOccurred())
			Expect(source.Fragment).To(Equal("Workflow"))

			schema, err := mcp.LoadCUEFromSource(ctx, source, "ci")
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
			violations, err := validator.CheckContract("policy.rego", policy, schema)
			Expect(err).NotTo(HaveOccurred())
			Expect(violations).To(BeEmpty(), "valid GitHub Actions paths should not produce violations")
		})

		It("returns error when fragment is missing on definitions-only module", func() {
			source, err := mcp.ParseSchemaSource("cue://cue.dev/x/githubactions@v0")
			Expect(err).NotTo(HaveOccurred())
			Expect(source.Fragment).To(BeEmpty())

			_, err = mcp.LoadCUEFromSource(ctx, source, "ci")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("#Workflow"))
		})
	})

	Describe("Embedded schema with top type and pattern constraints", func() {
		It("validates CI schema paths with top type", func() {
			data, err := schemas.GetBuiltInCUESchema("ci")
			Expect(err).NotTo(HaveOccurred())

			cueCtx := cuecontext.New()
			schema := cueCtx.CompileBytes(data)
			Expect(schema.Err()).NotTo(HaveOccurred())

			policy := `package ci.example
import rego.v1

deny contains msg if {
    input.on.push.branches
    input.on.pull_request.types
    input.on.schedule
    msg := "test"
}
`
			violations, err := validator.CheckContract("policy.rego", policy, schema)
			Expect(err).NotTo(HaveOccurred())
			Expect(violations).To(BeEmpty(), "top type paths should not produce violations")
		})

		It("validates CI schema paths with pattern constraints", func() {
			data, err := schemas.GetBuiltInCUESchema("ci")
			Expect(err).NotTo(HaveOccurred())

			cueCtx := cuecontext.New()
			schema := cueCtx.CompileBytes(data)
			Expect(schema.Err()).NotTo(HaveOccurred())

			policy := `package ci.example
import rego.v1

deny contains msg if {
    job := input.jobs.build
    job.steps
    job["runs-on"]
    msg := "test"
}
`
			violations, err := validator.CheckContract("policy.rego", policy, schema)
			Expect(err).NotTo(HaveOccurred())
			Expect(violations).To(BeEmpty(), "pattern constraint paths should not produce violations")
		})

		It("validates Kubernetes schema paths with map patterns", func() {
			data, err := schemas.GetBuiltInCUESchema("kubernetes")
			Expect(err).NotTo(HaveOccurred())

			cueCtx := cuecontext.New()
			schema := cueCtx.CompileBytes(data)
			Expect(schema.Err()).NotTo(HaveOccurred())

			policy := `package kubernetes.example
import rego.v1

deny contains msg if {
    input.metadata.labels.app
    input.metadata.annotations.owner
    msg := "test"
}
`
			violations, err := validator.CheckContract("policy.rego", policy, schema)
			Expect(err).NotTo(HaveOccurred())
			Expect(violations).To(BeEmpty(), "map pattern paths should not produce violations")
		})
	})
})
