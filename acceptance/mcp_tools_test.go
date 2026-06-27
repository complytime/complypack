// SPDX-License-Identifier: Apache-2.0

package acceptance_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/complytime/complypack/internal/mcp"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var _ = Describe("MCP Tools Integration", func() {
	var (
		server     *mcp.Server
		ctx        context.Context
		configPath string
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Create temporary config
		tmpDir := GinkgoT().TempDir()
		configPath = filepath.Join(tmpDir, "complypack.yaml")

		// Create minimal catalog for testing
		catalogPath := filepath.Join(tmpDir, "catalog.yaml")
		catalogContent := `metadata:
  id: test-catalog
  version: "1.0.0"
controls:
  - id: AC-1
    title: Test Control
    description: Test control description`

		err := os.WriteFile(catalogPath, []byte(catalogContent), 0600)
		Expect(err).ToNot(HaveOccurred())

		// Create config pointing to catalog (no schema source = use embedded)
		configContent := `evaluator-id: opa
version: 0.1.0
gemara:
  source: ` + catalogPath + `
schemas:
  - platform: kubernetes-pod`

		err = os.WriteFile(configPath, []byte(configContent), 0600)
		Expect(err).ToNot(HaveOccurred())

		// Create server
		opts := &mcp.ServerOptions{
			ConfigPath: configPath,
		}

		server, err = mcp.NewServer(ctx, opts)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("validate_policy tool", func() {
		It("should validate a valid policy", func() {
			validPolicy := `package main

import rego.v1

deny contains msg if {
	input.kind == "Pod"
	not input.metadata.name
	msg := "Pods must have a name"
}`

			input := map[string]interface{}{
				"policyContent": validPolicy,
				"platform":      "kubernetes-pod",
			}
			inputJSON, err := json.Marshal(input)
			Expect(err).ToNot(HaveOccurred())

			req := &mcpsdk.CallToolRequest{
				Params: &mcpsdk.CallToolParamsRaw{
					Name:      "validate_policy",
					Arguments: inputJSON,
				},
			}

			handler := mcp.GetValidatePolicyHandler(server)
			result, err := handler(ctx, req)
			Expect(err).ToNot(HaveOccurred())

			textContent, ok := result.Content[0].(*mcpsdk.TextContent)
			Expect(ok).To(BeTrue(), "Expected TextContent type")

			var response map[string]interface{}
			err = json.Unmarshal([]byte(textContent.Text), &response)
			Expect(err).ToNot(HaveOccurred())

			Expect(response["valid"]).To(BeTrue())
			Expect(response["syntaxErrors"]).To(BeEmpty())
			Expect(response["contractViolations"]).To(BeEmpty())
		})

		It("should detect syntax errors", func() {
			invalidPolicy := `package main

deny contains msg if {
	input.kind == "Pod"
	# Missing closing brace`

			input := map[string]interface{}{
				"policyContent": invalidPolicy,
				"platform":      "kubernetes-pod",
			}
			inputJSON, err := json.Marshal(input)
			Expect(err).ToNot(HaveOccurred())

			req := &mcpsdk.CallToolRequest{
				Params: &mcpsdk.CallToolParamsRaw{
					Name:      "validate_policy",
					Arguments: inputJSON,
				},
			}

			handler := mcp.GetValidatePolicyHandler(server)
			result, err := handler(ctx, req)
			Expect(err).ToNot(HaveOccurred())

			textContent, ok := result.Content[0].(*mcpsdk.TextContent)
			Expect(ok).To(BeTrue(), "Expected TextContent type")

			var response map[string]interface{}
			err = json.Unmarshal([]byte(textContent.Text), &response)
			Expect(err).ToNot(HaveOccurred())

			Expect(response["valid"]).To(BeFalse())
			syntaxErrors := response["syntaxErrors"].([]interface{})
			Expect(syntaxErrors).ToNot(BeEmpty())
		})

		It("should detect contract violations", func() {
			policyWithViolation := `package main

import rego.v1

deny contains msg if {
	input.kind == "Pod"
	not input.metadata.invalid_field
	msg := "Contract violation example"
}`

			input := map[string]interface{}{
				"policyContent": policyWithViolation,
				"platform":      "kubernetes-pod",
			}
			inputJSON, err := json.Marshal(input)
			Expect(err).ToNot(HaveOccurred())

			req := &mcpsdk.CallToolRequest{
				Params: &mcpsdk.CallToolParamsRaw{
					Name:      "validate_policy",
					Arguments: inputJSON,
				},
			}

			handler := mcp.GetValidatePolicyHandler(server)
			result, err := handler(ctx, req)
			Expect(err).ToNot(HaveOccurred())

			textContent, ok := result.Content[0].(*mcpsdk.TextContent)
			Expect(ok).To(BeTrue(), "Expected TextContent type")

			var response map[string]interface{}
			err = json.Unmarshal([]byte(textContent.Text), &response)
			Expect(err).ToNot(HaveOccurred())

			Expect(response["valid"]).To(BeFalse())
			violations := response["contractViolations"].([]interface{})
			Expect(violations).ToNot(BeEmpty())
		})
	})

	Describe("test_policy tool", func() {
		It("should validate test data and execute tests", func() {
			policy := `package main

import rego.v1

deny contains msg if {
	input.kind == "Pod"
	not input.metadata.labels["app"]
	msg := "Pods must have 'app' label"
}`

			testData := map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name":      "test-pod",
					"namespace": "default",
					"labels": map[string]interface{}{
						"app": "nginx",
					},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "nginx",
							"image": "nginx:latest",
						},
					},
				},
			}

			input := map[string]interface{}{
				"policyContent": policy,
				"testData":      testData,
				"platform":      "kubernetes-pod",
			}
			inputJSON, err := json.Marshal(input)
			Expect(err).ToNot(HaveOccurred())

			req := &mcpsdk.CallToolRequest{
				Params: &mcpsdk.CallToolParamsRaw{
					Name:      "test_policy",
					Arguments: inputJSON,
				},
			}

			handler := mcp.GetTestPolicyHandler(server)
			result, err := handler(ctx, req)
			Expect(err).ToNot(HaveOccurred())

			textContent, ok := result.Content[0].(*mcpsdk.TextContent)
			Expect(ok).To(BeTrue(), "Expected TextContent type")

			var response map[string]interface{}
			err = json.Unmarshal([]byte(textContent.Text), &response)
			Expect(err).ToNot(HaveOccurred())

			Expect(response["testDataValid"]).To(BeTrue())
			Expect(response["testsExecuted"]).To(BeTrue())
		})

		It("should handle test execution with any test data", func() {
			policy := `package main

import rego.v1

deny contains msg if {
	input.kind == "Pod"
	msg := "Test"
}`

			testData := map[string]interface{}{
				"kind": "Pod",
				"metadata": map[string]interface{}{
					"name": "test",
				},
			}

			input := map[string]interface{}{
				"policyContent": policy,
				"testData":      testData,
				"platform":      "kubernetes-pod",
			}
			inputJSON, err := json.Marshal(input)
			Expect(err).ToNot(HaveOccurred())

			req := &mcpsdk.CallToolRequest{
				Params: &mcpsdk.CallToolParamsRaw{
					Name:      "test_policy",
					Arguments: inputJSON,
				},
			}

			handler := mcp.GetTestPolicyHandler(server)
			result, err := handler(ctx, req)
			Expect(err).ToNot(HaveOccurred())

			textContent, ok := result.Content[0].(*mcpsdk.TextContent)
			Expect(ok).To(BeTrue(), "Expected TextContent type")

			var response map[string]interface{}
			err = json.Unmarshal([]byte(textContent.Text), &response)
			Expect(err).ToNot(HaveOccurred())

			// Verify response structure
			Expect(response).To(HaveKey("testDataValid"))
			Expect(response).To(HaveKey("testsExecuted"))
			Expect(response["testDataValid"]).To(BeTrue())
			Expect(response["testsExecuted"]).To(BeTrue())
		})
	})
})
