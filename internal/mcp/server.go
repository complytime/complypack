// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/evaluator"
	"github.com/complytime/complypack/internal/registry"
	"github.com/complytime/complypack/schemas"
	"github.com/gemaraproj/go-gemara"
	"github.com/gemaraproj/go-gemara/bundle"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
)

// Server wraps the MCP SDK server with ComplyPack-specific state.
type Server struct {
	mcp           *mcp.Server
	ResourceStore *ResourceStore
}

// ServerOptions configures ComplyPack MCP server initialization.
type ServerOptions struct {
	// ConfigPath is the path to complypack.yaml.
	ConfigPath string

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

	// Load config
	cfg, err := config.LoadConfig(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Load Gemara catalog(s) from source (file path or OCI reference)
	// OCI bundles may include imports/extends, so we get a map back
	artifacts, err := loadArtifacts(ctx, cfg.Gemara.Source, cfg.Gemara.PlainHTTP)
	if err != nil {
		return nil, fmt.Errorf("failed to load artifacts from %s: %w", cfg.Gemara.Source, err)
	}

	// Load schemas from configured sources
	schemaMap, err := loadSchemas(ctx, cfg.Schemas)
	if err != nil {
		return nil, fmt.Errorf("failed to load schemas: %w", err)
	}

	// Set up evaluator registry
	evalRegistry := opts.EvaluatorRegistry
	if evalRegistry == nil {
		evalRegistry = evaluator.DefaultRegistry()
	}

	// Create resource store with parsed artifacts
	store := NewResourceStore(
		artifacts.RawCatalogs,
		artifacts.Catalogs,
		artifacts.Policies,
		artifacts.EffectivePolicies,
		schemaMap,
		evalRegistry,
	)

	// Create MCP server
	impl := &mcp.Implementation{
		Name:    "complypack-mcp",
		Version: "0.1.0",
	}

	mcpServer := mcp.NewServer(impl, &mcp.ServerOptions{
		Instructions: "ComplyPack MCP Server - provides Gemara catalogs and platform schemas",
	})

	// Register catalog resources (from raw catalogs for MCP resource serving)
	for name := range artifacts.RawCatalogs {
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
// loadSchemas loads JSON schemas from configured sources.
// Falls back to embedded schemas if source loading fails.
func loadSchemas(ctx context.Context, schemaRefs []config.SchemaRef) (map[string][]byte, error) {
	schemaMap := make(map[string][]byte)

	for _, ref := range schemaRefs {
		platform := ref.Platform

		// Determine source (new field takes precedence over legacy path)
		source := ref.Source
		if source == "" && ref.Path != "" {
			// Legacy path field - convert to file:// source
			source = "file://" + ref.Path
		}

		// Try loading from configured source
		var data []byte
		var err error

		if source != "" {
			parsed, parseErr := ParseSchemaSource(source)
			if parseErr != nil {
				return nil, fmt.Errorf("failed to parse schema source for %s: %w", platform, parseErr)
			}

			data, err = loadJSONSchemaFromSource(ctx, parsed, platform)
			if err != nil {
				slog.Warn("failed to load schema from source, falling back to embedded",
					"platform", platform, "source", source, "error", err)
			}
		}

		// Fallback to embedded if source failed or not specified
		if data == nil {
			data, err = schemas.GetBuiltInSchema(platform)
			if err != nil {
				slog.Warn("no schema available for platform, skipping",
					"platform", platform, "error", err)
				continue
			}
			slog.Info("loaded embedded schema", "platform", platform)
		}

		schemaMap[platform] = data
	}

	return schemaMap, nil
}

// loadJSONSchemaFromSource loads a JSON schema from a parsed source.
func loadJSONSchemaFromSource(ctx context.Context, source SchemaSource, platform string) ([]byte, error) {
	var data []byte
	var format SchemaFormat
	var err error

	switch source.Type {
	case SourceTypeHTTPS, SourceTypeHTTP:
		data, format, err = fetchSchemaFromURL(ctx, source.Path)
	case SourceTypeFile, SourceTypeLegacyPath:
		data, format, err = loadSchemaFromFile(source.Path)
	case SourceTypeCUEModule:
		// Load CUE schema and convert to JSON Schema
		return loadJSONSchemaFromCUE(ctx, source, platform)
	case SourceTypeUnknown:
		return nil, fmt.Errorf("no source specified")
	default:
		return nil, fmt.Errorf("unsupported source type: %v", source.Type)
	}

	if err != nil {
		return nil, err
	}

	if format != FormatJSON {
		return nil, fmt.Errorf("expected JSON format, got %v", format)
	}

	slog.Info("loaded schema from source", "platform", platform, "source", source.Path)
	return data, nil
}

// loadJSONSchemaFromCUE loads a CUE schema and converts it to JSON Schema.
func loadJSONSchemaFromCUE(ctx context.Context, source SchemaSource, platform string) ([]byte, error) {
	// Load the CUE schema using the existing loader
	cueVal, err := loadCUEFromSource(ctx, source, platform)
	if err != nil {
		return nil, fmt.Errorf("failed to load CUE schema: %w", err)
	}

	// Convert CUE value to JSON Schema
	// CUE's approach: export the value as JSON which represents the schema structure
	jsonBytes, err := cueVal.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to convert CUE to JSON: %w", err)
	}

	slog.Info("loaded and converted CUE schema to JSON", "platform", platform, "source", source.Path)
	return jsonBytes, nil
}

// LoadedArtifacts holds raw and parsed artifacts from bundle/file loading.
type LoadedArtifacts struct {
	RawCatalogs       map[string][]byte
	Catalogs          map[string]*gemara.ControlCatalog
	Policies          map[string]*gemara.Policy
	EffectivePolicies map[string]*gemara.EffectivePolicy
}

// loadArtifacts loads and classifies Gemara artifacts from either a file path or OCI reference.
// For OCI references, it returns both the primary artifact and any imports (bundle).
// For file paths, it returns the single artifact.
func loadArtifacts(ctx context.Context, source string, plainHTTP bool) (*LoadedArtifacts, error) {
	// Parse URI scheme
	if strings.HasPrefix(source, "file://") {
		// file:// URI - strip scheme and load local file
		path := strings.TrimPrefix(source, "file://")
		return loadFileArtifacts(ctx, path)
	}

	if strings.HasPrefix(source, "oci://") {
		// oci:// URI - strip scheme and pull from OCI registry
		ref := strings.TrimPrefix(source, "oci://")
		return loadBundleArtifacts(ctx, ref, plainHTTP)
	}

	// Legacy: No scheme - detect OCI vs file path
	if isOCIReference(source) {
		// Pull from OCI registry - returns primary + imports
		return loadBundleArtifacts(ctx, source, plainHTTP)
	}

	// Load from local file path - single artifact
	return loadFileArtifacts(ctx, source)
}

// loadFileArtifacts loads and classifies a single artifact from a file.
func loadFileArtifacts(ctx context.Context, path string) (*LoadedArtifacts, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Classify the artifact using gemara.Classify
	artifactSet, err := gemara.Classify(data)
	if err != nil {
		return nil, fmt.Errorf("failed to classify artifact: %w", err)
	}

	result := &LoadedArtifacts{
		RawCatalogs:       make(map[string][]byte),
		Catalogs:          make(map[string]*gemara.ControlCatalog),
		Policies:          make(map[string]*gemara.Policy),
		EffectivePolicies: make(map[string]*gemara.EffectivePolicy),
	}

	// Store catalogs
	for _, catalog := range artifactSet.ControlCatalogs {
		result.RawCatalogs[catalog.Metadata.Id] = data
		result.Catalogs[catalog.Metadata.Id] = &catalog
	}

	// Store policies and resolve effective policies
	for _, policy := range artifactSet.Policies {
		result.RawCatalogs[policy.Metadata.Id] = data
		result.Policies[policy.Metadata.Id] = &policy

		// Resolve effective policy if there are catalogs/guidance
		if len(artifactSet.ControlCatalogs) > 0 || len(artifactSet.GuidanceCatalogs) > 0 {
			effective, err := gemara.ResolveEffectivePolicy(policy, artifactSet.ControlCatalogs, artifactSet.GuidanceCatalogs)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve effective policy: %w", err)
			}
			result.EffectivePolicies[policy.Metadata.Id] = effective
		}
	}

	return result, nil
}

// loadBundleArtifacts loads and classifies artifacts from an OCI bundle.
func loadBundleArtifacts(ctx context.Context, ref string, plainHTTP bool) (*LoadedArtifacts, error) {
	// Get Docker credentials
	credFunc, err := registry.NewCredentialFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to load Docker credentials: %w", err)
	}

	// Create remote repository
	repo, err := registry.NewRepository(ref, credFunc, plainHTTP)
	if err != nil {
		return nil, err
	}

	// Extract tag from reference
	tag := registry.ParseTag(ref)

	// Resolve and pull manifest
	store := memory.New()
	_, err = oras.Copy(ctx, repo, tag, store, tag, oras.DefaultCopyOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to pull from registry: %w", err)
	}

	// Unpack the Gemara bundle
	b, err := bundle.Unpack(ctx, store, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack bundle: %w", err)
	}

	// Classify the bundle
	classified, err := b.Classify()
	if err != nil {
		return nil, fmt.Errorf("failed to classify bundle: %w", err)
	}

	result := &LoadedArtifacts{
		RawCatalogs:       make(map[string][]byte),
		Catalogs:          make(map[string]*gemara.ControlCatalog),
		Policies:          make(map[string]*gemara.Policy),
		EffectivePolicies: make(map[string]*gemara.EffectivePolicy),
	}

	// Store primary policy if present
	if classified.Policy != nil {
		result.RawCatalogs[classified.Policy.Metadata.Id] = b.Files[0].Data
		result.Policies[classified.Policy.Metadata.Id] = classified.Policy
	}

	// Store primary control catalog if present
	if classified.ControlCatalog != nil {
		result.RawCatalogs[classified.ControlCatalog.Metadata.Id] = b.Files[0].Data
		result.Catalogs[classified.ControlCatalog.Metadata.Id] = classified.ControlCatalog
	}

	// Store import catalogs
	if classified.Imports != nil {
		for _, catalog := range classified.Imports.ControlCatalogs {
			// Find corresponding raw data from bundle imports
			for _, imp := range b.Imports {
				if imp.Type == "ControlCatalog" {
					name, err := extractCatalogName(imp.Data)
					if err == nil && name == catalog.Metadata.Id {
						result.RawCatalogs[catalog.Metadata.Id] = imp.Data
						break
					}
				}
			}
			result.Catalogs[catalog.Metadata.Id] = &catalog
		}
	}

	// Resolve effective policy if we have a policy
	if classified.Policy != nil {
		var catalogs []gemara.ControlCatalog
		var guidance []gemara.GuidanceCatalog

		if classified.ControlCatalog != nil {
			catalogs = append(catalogs, *classified.ControlCatalog)
		}
		if classified.Imports != nil {
			catalogs = append(catalogs, classified.Imports.ControlCatalogs...)
			guidance = append(guidance, classified.Imports.GuidanceCatalogs...)
		}

		effective, err := gemara.ResolveEffectivePolicy(*classified.Policy, catalogs, guidance)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve effective policy: %w", err)
		}
		result.EffectivePolicies[classified.Policy.Metadata.Id] = effective
	}

	return result, nil
}

// isOCIReference returns true if the source looks like an OCI reference.
func isOCIReference(source string) bool {
	// OCI references contain a registry host (domain with optional port)
	// Examples: ghcr.io/org/repo:tag, localhost:5000/repo:tag, http://registry/repo
	return strings.Contains(source, "/") && (strings.Contains(source, ":") || strings.Contains(source, "//"))
}

// pullCatalogsFromOCI pulls a Gemara catalog and its imports from an OCI registry.
// Returns a map of catalog name -> catalog data (YAML bytes).
