// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"cuelang.org/go/cue"
	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/internal/requirement"
	"github.com/complytime/complypack/internal/schema"
	"github.com/complytime/complypack/internal/source"
	"github.com/complytime/complypack/internal/version"
	"github.com/complytime/complypack/schemas"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP SDK server with ComplyPack-specific state.
type Server struct {
	mcp           *mcp.Server
	ResourceStore *ResourceStore
}

// ServerOptions configures ComplyPack MCP server initialization.
type ServerOptions struct {
	// ConfigPath is the path to complypack.yaml.
	// Ignored when Config is set.
	ConfigPath string

	// Config provides configuration directly, bypassing file loading.
	// When set, ConfigPath is ignored.
	Config *config.ComplyPackConfig

	// OCIStore is the directory for OCI artifact caching.
	OCIStore string

	// CacheDir is the directory for MCP server caching.
	CacheDir string

	// EvaluatorRegistry provides available policy evaluators.
	// If nil, defaults to evaluator.DefaultRegistry().
	EvaluatorRegistry *evaluator.Registry
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

	// Load config: use provided config or load from file
	var cfg *config.ComplyPackConfig
	if opts.Config != nil {
		cfg = opts.Config
	} else {
		var err error
		cfg, err = config.LoadConfig(opts.ConfigPath, false, os.Stderr)
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	}
	if err := cfg.ValidateForMCP(); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Load Gemara artifacts from all configured sources
	loaded := requirement.NewArtifactSet()
	for _, entry := range cfg.Gemara.Sources {
		src, err := source.LoadArtifacts(ctx, entry.Source, entry.PlainHTTP, opts.CacheDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load artifacts from %s: %w", entry.Source, err)
		}
		if err := loaded.Merge(src); err != nil {
			return nil, fmt.Errorf("failed to merge artifacts from %s: %w", entry.Source, err)
		}
	}

	// Resolve effective policies
	resolved := make(map[string]*requirement.ResolvedPolicy)
	for id, policy := range loaded.Policies {
		if len(loaded.Catalogs) > 0 || len(loaded.Guidance) > 0 {
			rp, err := requirement.ResolvePolicy(*policy, loaded)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve effective policy %s: %w", id, err)
			}
			resolved[id] = rp
		}
	}

	// Build unified artifact map for MCP resource serving (marshal on demand)
	allArtifacts := make(map[string]any)
	for id, c := range loaded.Catalogs {
		allArtifacts[id] = c
	}
	for id, gc := range loaded.Guidance {
		allArtifacts[id] = gc
	}
	for id, p := range loaded.Policies {
		allArtifacts[id] = p
	}
	for id, md := range loaded.Mappings {
		allArtifacts[id] = md
	}

	// Load schemas from configured sources
	schemaReg := schema.DefaultRegistry()
	schemaMap, cueSchemaMap, err := loadSchemas(ctx, cfg.Schemas, schemaReg)
	if err != nil {
		return nil, fmt.Errorf("failed to load schemas: %w", err)
	}

	// Set up evaluator registry
	evalRegistry := opts.EvaluatorRegistry
	if evalRegistry == nil {
		evalRegistry = evaluator.DefaultRegistry()
	}

	store := NewResourceStore(
		allArtifacts,
		resolved,
		schemaMap,
		cueSchemaMap,
		evalRegistry,
	)

	// Create MCP server
	impl := &mcp.Implementation{
		Name:    "complypack-mcp",
		Version: version.ModuleVersion(),
	}

	mcpServer := mcp.NewServer(impl, &mcp.ServerOptions{
		Instructions: "ComplyPack MCP Server - provides Gemara catalogs and platform schemas",
	})

	// Register artifact resources
	for name := range allArtifacts {
		uri := fmt.Sprintf("%s://%s/%s", URIScheme, ResourceTypeCatalog, name)
		resource := &mcp.Resource{
			URI:      uri,
			Name:     fmt.Sprintf("Gemara Artifact: %s", name),
			MIMEType: MIMETypeYAML,
		}
		mcpServer.AddResource(resource, createResourceHandler(store, uri))
	}

	// Register mapping document resources
	for name := range loaded.Mappings {
		uri := fmt.Sprintf("%s://%s/%s", URIScheme, ResourceTypeMapping, name)
		resource := &mcp.Resource{
			URI:      uri,
			Name:     fmt.Sprintf("Gemara Mapping Document: %s", name),
			MIMEType: MIMETypeYAML,
		}
		mcpServer.AddResource(resource, createResourceHandler(store, uri))
	}

	// Register schema list resource (discovery)
	schemaListURI := fmt.Sprintf("%s://%s", URIScheme, ResourceTypeSchema)
	mcpServer.AddResource(&mcp.Resource{
		URI:      schemaListURI,
		Name:     "Available Platform Schemas",
		MIMEType: MIMETypeJSON,
	}, createResourceHandler(store, schemaListURI))

	// Register per-platform schema resources
	for platform := range schemaMap {
		uri := fmt.Sprintf("%s://%s/%s", URIScheme, ResourceTypeSchema, platform)
		mime := MIMETypeCUE
		if schema.IsJSONSchema(schemaMap[platform]) {
			mime = MIMETypeJSONSchema
		}
		resource := &mcp.Resource{
			URI:      uri,
			Name:     fmt.Sprintf("Platform Schema: %s", platform),
			MIMEType: mime,
		}
		mcpServer.AddResource(resource, createResourceHandler(store, uri))
	}

	// Register evaluator resource
	evalURI := fmt.Sprintf("%s://%s", URIScheme, ResourceTypeEvaluator)
	evalResource := &mcp.Resource{
		URI:      evalURI,
		Name:     "Available Policy Evaluators",
		MIMEType: MIMETypeJSON,
	}
	mcpServer.AddResource(evalResource, createResourceHandler(store, evalURI))

	// Register tools
	validateTool := createValidatePolicyTool()
	mcpServer.AddTool(validateTool, handleValidatePolicy(store))

	testTool := createTestPolicyTool()
	mcpServer.AddTool(testTool, handleTestPolicy(store))

	assessmentTool := createGetAssessmentRequirementsTool()
	mcpServer.AddTool(assessmentTool, handleGetAssessmentRequirements(store))

	deltaTool := createAnalyzeParameterDeltaTool()
	mcpServer.AddTool(deltaTool, handleAnalyzeParameterDelta(store, loaded))

	triageTool := createGetAutomationTriageTool()
	mcpServer.AddTool(triageTool, handleGetAutomationTriage(store))

	applicabilityTool := createGetApplicabilityGroupsTool()
	mcpServer.AddTool(applicabilityTool, handleGetApplicabilityGroups(store))

	return &Server{
		mcp:           mcpServer,
		ResourceStore: store,
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

// Run starts the MCP server on the given transport.
// It delegates to the underlying MCP SDK server's Run method.
func (s *Server) Run(ctx context.Context, transport mcp.Transport) error {
	return s.mcp.Run(ctx, transport)
}

// loadSchemas loads platform schemas via the schema registry.
// If a schema ref has no explicit source, it checks the schema index for a default.
func loadSchemas(ctx context.Context, schemaRefs []config.SchemaRef, reg *schema.Registry) (map[string][]byte, map[string]cue.Value, error) {
	index, err := schemas.LoadIndex()
	if err != nil {
		return nil, nil, fmt.Errorf("loading schema index: %w", err)
	}

	schemaMap := make(map[string][]byte)
	cueSchemaMap := make(map[string]cue.Value)

	for _, ref := range schemaRefs {
		platform := ref.Platform
		source := schemas.ResolveSource(ref, index)

		s, err := reg.Load(ctx, source, platform)
		if err != nil {
			if source == "" {
				slog.Warn("no schema available for platform, skipping",
					"platform", platform, "error", err)
				continue
			}
			return nil, nil, fmt.Errorf("failed to load schema for platform %s from %s: %w", platform, source, err)
		}

		schemaMap[platform] = s.Bytes
		cueSchemaMap[platform] = s.CUE
		slog.Info("loaded schema", "platform", platform, "source", source)
	}

	return schemaMap, cueSchemaMap, nil
}
