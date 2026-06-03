// SPDX-License-Identifier: Apache-2.0

package acceptance_test

import (
	"context"
	"os"
	"path/filepath"

	"github.com/complytime/complypack/internal/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MCP Server", func() {
	var (
		ctx        context.Context
		tempDir    string
		configPath string
		catalogDir string
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Create temporary directories
		var err error
		tempDir, err = os.MkdirTemp("", "complypack-acceptance-*")
		Expect(err).NotTo(HaveOccurred())

		catalogDir = filepath.Join(tempDir, "catalogs")
		err = os.MkdirAll(catalogDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		// Create a test catalog
		catalogPath := filepath.Join(catalogDir, "test-catalog.yaml")
		catalogContent := `metadata:
  id: test-catalog
  version: 1.0.0
  gemara-version: 0.20.0
  type: ControlCatalog
controls:
  - id: AC-1
    title: Access Control Policy
    description: Develop and maintain access control policy.
  - id: AC-2
    title: Account Management
    description: Manage information system accounts.
`
		err = os.WriteFile(catalogPath, []byte(catalogContent), 0600)
		Expect(err).NotTo(HaveOccurred())

		// Create config file
		configPath = filepath.Join(tempDir, "complypack.yaml")
		configContent := `evaluator-id: io.complytime.opa
version: 0.1.0
gemara:
  source: ` + catalogPath + `
schemas:
  - path: schemas/kubernetes.cue
    platform: kubernetes
  - path: schemas/terraform.cue
    platform: terraform
`
		err = os.WriteFile(configPath, []byte(configContent), 0600)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Describe("Server Initialization", func() {
		It("should create a server with valid configuration", func() {
			opts := &mcp.ServerOptions{
				ConfigPath: configPath,
				CacheDir:   filepath.Join(tempDir, "cache"),
			}

			server, err := mcp.NewServer(ctx, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(server).NotTo(BeNil())
		})

		It("should fail with missing config file", func() {
			opts := &mcp.ServerOptions{
				ConfigPath: "/nonexistent/config.yaml",
				CacheDir:   filepath.Join(tempDir, "cache"),
			}

			server, err := mcp.NewServer(ctx, opts)
			Expect(err).To(HaveOccurred())
			Expect(server).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to load config"))
		})

		It("should fail with missing catalog file", func() {
			badConfigPath := filepath.Join(tempDir, "bad-config.yaml")
			badConfigContent := `evaluator-id: io.complytime.opa
version: 0.1.0
gemara:
  source: /nonexistent/catalog.yaml
schemas:
  - path: schemas/kubernetes.cue
    platform: kubernetes
`
			err := os.WriteFile(badConfigPath, []byte(badConfigContent), 0600)
			Expect(err).NotTo(HaveOccurred())

			opts := &mcp.ServerOptions{
				ConfigPath: badConfigPath,
				CacheDir:   filepath.Join(tempDir, "cache"),
			}

			server, err := mcp.NewServer(ctx, opts)
			Expect(err).To(HaveOccurred())
			Expect(server).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to load artifacts"))
		})

		It("should skip unknown platform without failing", func() {
			unknownConfigPath := filepath.Join(tempDir, "unknown-platform-config.yaml")
			catalogPath := filepath.Join(catalogDir, "test-catalog.yaml")
			unknownConfigContent := `evaluator-id: io.complytime.opa
version: 0.1.0
gemara:
  source: ` + catalogPath + `
schemas:
  - path: schemas/invalid.cue
    platform: unsupported-platform
`
			err := os.WriteFile(unknownConfigPath, []byte(unknownConfigContent), 0600)
			Expect(err).NotTo(HaveOccurred())

			opts := &mcp.ServerOptions{
				ConfigPath: unknownConfigPath,
				CacheDir:   filepath.Join(tempDir, "cache"),
			}

			server, err := mcp.NewServer(ctx, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(server).NotTo(BeNil())
		})

		// Removed: duplicate catalog test - no longer applicable with single source config
	})

	Describe("Resource Operations", func() {
		var (
			server *mcp.Server
			store  *mcp.ResourceStore
		)

		BeforeEach(func() {
			opts := &mcp.ServerOptions{
				ConfigPath: configPath,
				CacheDir:   filepath.Join(tempDir, "cache"),
			}

			var err error
			server, err = mcp.NewServer(ctx, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(server).NotTo(BeNil())

			// Access the resource store through reflection or exported field
			// Note: ResourceStore needs to be exported in the server for this to work
			// For now, we'll create a separate store for testing
			catalogs := map[string][]byte{
				"test-catalog": []byte(`metadata:
  id: test-catalog
  version: 1.0.0
controls:
  - id: AC-1
    title: Test Control
`),
			}

			// Load schemas (simplified - in real code this comes from schemas package)
			schemas := map[string][]byte{
				"kubernetes": []byte(`{"type": "object"}`),
			}

			store = mcp.NewResourceStore(catalogs, nil, nil, nil, schemas)
		})

		It("should list all catalog and schema resources", func() {
			resources, err := store.ListResources(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(resources).NotTo(BeEmpty())

			// Should have at least one catalog and one schema
			var hasCatalog, hasSchema bool
			for _, res := range resources {
				if res.MIMEType == "application/yaml" {
					hasCatalog = true
				}
				if res.MIMEType == "application/json" {
					hasSchema = true
				}
			}
			Expect(hasCatalog).To(BeTrue(), "should have at least one catalog resource")
			Expect(hasSchema).To(BeTrue(), "should have at least one schema resource")
		})

		It("should read a catalog resource", func() {
			uri := "complypack://catalog/test-catalog"
			contents, err := store.ReadResource(ctx, uri)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).NotTo(BeEmpty())
			Expect(contents[0].URI).To(Equal(uri))
			Expect(contents[0].MIMEType).To(Equal("application/yaml"))
			Expect(contents[0].Text).To(ContainSubstring("test-catalog"))
		})

		It("should read a schema resource", func() {
			uri := "complypack://schema/kubernetes"
			contents, err := store.ReadResource(ctx, uri)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).NotTo(BeEmpty())
			Expect(contents[0].URI).To(Equal(uri))
			Expect(contents[0].MIMEType).To(Equal("application/json"))
			Expect(contents[0].Text).NotTo(BeEmpty())
		})

		It("should fail to read non-existent catalog", func() {
			uri := "complypack://catalog/nonexistent"
			contents, err := store.ReadResource(ctx, uri)
			Expect(err).To(HaveOccurred())
			Expect(contents).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("should fail to read non-existent schema", func() {
			uri := "complypack://schema/nonexistent"
			contents, err := store.ReadResource(ctx, uri)
			Expect(err).To(HaveOccurred())
			Expect(contents).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("should fail with invalid URI scheme", func() {
			uri := "invalid://catalog/test"
			contents, err := store.ReadResource(ctx, uri)
			Expect(err).To(HaveOccurred())
			Expect(contents).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("invalid URI scheme"))
		})

		It("should fail with invalid URI format", func() {
			uri := "complypack://invalid-format"
			contents, err := store.ReadResource(ctx, uri)
			Expect(err).To(HaveOccurred())
			Expect(contents).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("invalid URI format"))
		})

		It("should fail with unknown resource type", func() {
			uri := "complypack://unknown/test"
			contents, err := store.ReadResource(ctx, uri)
			Expect(err).To(HaveOccurred())
			Expect(contents).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("unknown resource type"))
		})
	})

	Describe("End-to-End Workflow", func() {
		It("should initialize server and provide catalog resources", func() {
			opts := &mcp.ServerOptions{
				ConfigPath: configPath,
				CacheDir:   filepath.Join(tempDir, "cache"),
			}

			// Create server
			server, err := mcp.NewServer(ctx, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(server).NotTo(BeNil())

			// Create a test store with the same data
			catalogContent, err := os.ReadFile(filepath.Join(catalogDir, "test-catalog.yaml"))
			Expect(err).NotTo(HaveOccurred())

			catalogs := map[string][]byte{
				"test-catalog": catalogContent,
			}
			schemas := map[string][]byte{
				"kubernetes": []byte(`{"type": "object"}`),
			}
			store := mcp.NewResourceStore(catalogs, nil, nil, nil, schemas)

			// List resources
			resources, err := store.ListResources(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(resources).NotTo(BeEmpty())

			// Find catalog resource
			var catalogURI string
			for _, res := range resources {
				if res.MIMEType == "application/yaml" && res.Name == "Gemara Catalog: test-catalog" {
					catalogURI = res.URI
					break
				}
			}
			Expect(catalogURI).NotTo(BeEmpty())

			// Read catalog
			contents, err := store.ReadResource(ctx, catalogURI)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).NotTo(BeEmpty())
			Expect(contents[0].Text).To(ContainSubstring("AC-1"))
			Expect(contents[0].Text).To(ContainSubstring("Access Control Policy"))
		})

		It("should provide platform schemas", func() {
			opts := &mcp.ServerOptions{
				ConfigPath: configPath,
				CacheDir:   filepath.Join(tempDir, "cache"),
			}

			// Create server
			server, err := mcp.NewServer(ctx, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(server).NotTo(BeNil())

			// Create a test store
			schemas := map[string][]byte{
				"kubernetes": []byte(`{"type": "object", "properties": {"kind": {"type": "string"}}}`),
			}
			store := mcp.NewResourceStore(map[string][]byte{}, nil, nil, nil, schemas)

			// List resources
			resources, err := store.ListResources(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Find schema resource
			var schemaURI string
			for _, res := range resources {
				if res.MIMEType == "application/json" && res.Name == "Platform Schema: kubernetes" {
					schemaURI = res.URI
					break
				}
			}
			Expect(schemaURI).NotTo(BeEmpty())

			// Read schema
			contents, err := store.ReadResource(ctx, schemaURI)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).NotTo(BeEmpty())
			Expect(contents[0].MIMEType).To(Equal("application/json"))
			Expect(contents[0].Text).To(ContainSubstring("object"))
		})
	})
})
