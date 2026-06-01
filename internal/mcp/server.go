// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"os"

	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/schemas"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"
)

// Server wraps the MCP SDK server with ComplyPack-specific state.
type Server struct {
	mcp           *mcp.Server
	resourceStore *ResourceStore
}

// ServerOptions configures ComplyPack MCP server initialization.
type ServerOptions struct {
	// ConfigPath is the path to complypack.yaml.
	ConfigPath string

	// OCIStore is the directory for OCI artifact caching (unused for now, catalogs come from local paths).
	OCIStore string

	// CacheDir is the directory for MCP server caching.
	CacheDir string

	// PlainHTTP forces HTTP instead of HTTPS for OCI registry (unused for now).
	PlainHTTP bool
}

// NewServer creates a ComplyPack MCP server.
// It loads the config, reads catalogs from local paths, loads platform schemas,
// validates the platform, and creates the MCP server with resource handlers.
//
// Fails fast if:
// - Config file cannot be loaded or parsed
// - Any catalog file cannot be read
// - Platform is not supported
// - Duplicate catalog names are detected
func NewServer(ctx context.Context, opts *ServerOptions) (*Server, error) {
	if opts == nil {
		return nil, fmt.Errorf("ServerOptions cannot be nil")
	}

	// Load config
	cfg, err := config.LoadConfig(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Load Gemara catalog from source
	catalogs := make(map[string][]byte)
	data, err := os.ReadFile(cfg.Gemara.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to read catalog at %s: %w", cfg.Gemara.Source, err)
	}

	// Extract catalog name from metadata.id
	catalogName, err := extractCatalogName(data)
	if err != nil {
		return nil, fmt.Errorf("failed to extract catalog name from %s: %w", cfg.Gemara.Source, err)
	}

	catalogs[catalogName] = data

	// Load all built-in platform schemas
	schemaMap, err := loadSchemas()
	if err != nil {
		return nil, fmt.Errorf("failed to load schemas: %w", err)
	}

	// Validate that configured schemas reference valid platforms
	for _, schemaRef := range cfg.Schemas {
		if _, ok := schemaMap[schemaRef.Platform]; !ok {
			return nil, fmt.Errorf("schema references unsupported platform %q (available: %v)", schemaRef.Platform, schemas.BuiltInPlatforms)
		}
	}

	// Create resource store
	store := NewResourceStore(catalogs, schemaMap)

	// Create MCP server
	impl := &mcp.Implementation{
		Name:    "complypack-mcp",
		Version: "0.1.0",
	}

	mcpServer := mcp.NewServer(impl, &mcp.ServerOptions{
		Instructions: "ComplyPack MCP Server - provides Gemara catalogs and platform schemas",
	})

	// Register catalog resources
	for name := range catalogs {
		uri := fmt.Sprintf("%s://%s/%s", URIScheme, ResourceTypeCatalog, name)
		resource := &mcp.Resource{
			URI:      uri,
			Name:     fmt.Sprintf("Gemara Catalog: %s", name),
			MIMEType: MIMETypeYAML,
		}
		mcpServer.AddResource(resource, createResourceHandler(store, uri))
	}

	// Register schema resources
	for platform := range schemaMap {
		uri := fmt.Sprintf("%s://%s/%s", URIScheme, ResourceTypeSchema, platform)
		resource := &mcp.Resource{
			URI:      uri,
			Name:     fmt.Sprintf("Platform Schema: %s", platform),
			MIMEType: MIMETypeJSONSchema,
		}
		mcpServer.AddResource(resource, createResourceHandler(store, uri))
	}

	return &Server{
		mcp:           mcpServer,
		resourceStore: store,
	}, nil
}

// createResourceHandler creates a ResourceHandler that reads from the ResourceStore.
func createResourceHandler(store *ResourceStore, uri string) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		contents, err := store.ReadResource(ctx, req.Params.URI)
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{Contents: contents}, nil
	}
}

// extractCatalogName parses the catalog YAML and extracts metadata.id.
// Returns error if YAML is invalid or metadata.id is missing.
func extractCatalogName(data []byte) (string, error) {
	var parsed struct {
		Metadata struct {
			ID string `yaml:"id"`
		} `yaml:"metadata"`
	}

	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse catalog YAML: %w", err)
	}

	if parsed.Metadata.ID == "" {
		return "", fmt.Errorf("catalog missing metadata.id field")
	}

	return parsed.Metadata.ID, nil
}

// Run starts the MCP server on the given transport.
// It delegates to the underlying MCP SDK server's Run method.
func (s *Server) Run(ctx context.Context, transport mcp.Transport) error {
	return s.mcp.Run(ctx, transport)
}

// loadSchemas loads all built-in platform schemas.
func loadSchemas() (map[string][]byte, error) {
	schemaMap := make(map[string][]byte)

	for _, platform := range schemas.BuiltInPlatforms {
		data, err := schemas.GetBuiltInSchema(platform)
		if err != nil {
			return nil, fmt.Errorf("failed to load schema for %s: %w", platform, err)
		}
		schemaMap[platform] = data
	}

	return schemaMap, nil
}
