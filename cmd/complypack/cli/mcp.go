// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/complytime/complypack/internal/cache"
	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/mcp"
	"github.com/complytime/complypack/schemas/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// mcpCmd creates the "mcp" command.
func mcpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
		Long:  "Commands for running the ComplyPack Model Context Protocol (MCP) server",
	}

	cmd.AddCommand(mcpServeCmd())

	return cmd
}

// mcpServeCmd creates the "mcp serve" command.
func mcpServeCmd() *cobra.Command {
	var (
		configPath string
		cacheDir   string
		sources    []string
		schemas    []string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the ComplyPack MCP server",
		Long: `Start the ComplyPack MCP server on stdio transport.

The MCP server provides Gemara catalogs and platform schemas as resources
to MCP clients like Claude Desktop. It reads catalogs from local file paths
specified in complypack.yaml.

Example:
  complypack mcp serve --config complypack.yaml

  # Or use flags directly (no config file needed):
  complypack mcp serve \
    --source oci://ghcr.io/org/catalog:v1 \
    --schema kubernetes \
    --schema ci=cue://cue.dev/x/githubactions@v0#Workflow

The server runs until interrupted (Ctrl+C) or the client disconnects.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Resolve cache directory using XDG_CACHE_HOME-aware resolution
			resolvedCacheDir, err := cache.ResolveDir(cacheDir)
			if err != nil {
				return fmt.Errorf("failed to resolve cache directory: %w", err)
			}

			// Create MCP server options
			opts := &mcp.ServerOptions{
				CacheDir: resolvedCacheDir,
			}

			// If any CLI flags are present, build config from flags
			if len(sources) > 0 || len(schemas) > 0 {
				cfg, err := buildConfigFromFlags(sources, schemas)
				if err != nil {
					writeStartupError(err)
					return fmt.Errorf("failed to build config from flags: %w", err)
				}
				opts.Config = cfg
			} else {
				opts.ConfigPath = configPath
			}

			server, err := mcp.NewServer(ctx, opts)
			if err != nil {
				writeStartupError(err)
				return fmt.Errorf("failed to create MCP server: %w", err)
			}

			// Run server on stdio transport
			log.Printf("Starting ComplyPack MCP server...")
			if err := server.Run(ctx, &mcpsdk.StdioTransport{}); err != nil {
				return fmt.Errorf("MCP server failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "complypack.yaml", "Path to complypack.yaml config file")
	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", cache.CacheDirHelp)
	cmd.Flags().StringArrayVar(&sources, "source", nil, "Gemara OCI source (repeatable, e.g. oci://ghcr.io/org/catalog:v1)")
	cmd.Flags().StringArrayVar(&schemas, "schema", nil, "Platform schema (repeatable, e.g. kubernetes or ci=cue://...)")

	return cmd
}

// buildConfigFromFlags creates a ComplyPackConfig from --source and --schema flag values.
func buildConfigFromFlags(sources, schemas []string) (*config.ComplyPackConfig, error) {
	entries, err := parseSourceFlags(sources)
	if err != nil {
		return nil, err
	}

	schemaRefs, err := parseSchemaFlags(schemas)
	if err != nil {
		return nil, err
	}

	cfg := config.BuildConfig("", "", "", entries, schemaRefs)

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshaling config: %w", err)
	}

	if _, err := jsonschema.ValidateConfig(data, false); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

// parseSourceFlags converts --source flag values into GemaraSourceEntry values.
//
//   - oci://...        -> GemaraSourceEntry{Source: "oci://...", PlainHTTP: false}
//   - oci+http://...   -> GemaraSourceEntry{Source: "oci://...", PlainHTTP: true}
func parseSourceFlags(sources []string) ([]config.GemaraSourceEntry, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	entries := make([]config.GemaraSourceEntry, 0, len(sources))
	for _, s := range sources {
		if s == "" {
			return nil, fmt.Errorf("empty source flag value")
		}

		entry := config.GemaraSourceEntry{}
		if strings.HasPrefix(s, "oci+http://") {
			entry.Source = "oci://" + strings.TrimPrefix(s, "oci+http://")
			entry.PlainHTTP = true
		} else {
			entry.Source = s
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// writeStartupError writes a JSON-RPC error response to stdout so MCP clients
// can surface the real error message to the user. Without this, clients that
// communicate over stdio only see the pipe close and report a generic
// "error -32000: Connection closed" with no diagnostic context.
//
// Per the JSON-RPC 2.0 spec, when an error occurs before a request id can be
// determined, the response id MUST be null. This is written as raw JSON to
// avoid SDK-level restrictions on null-id responses.
func writeStartupError(err error) {
	resp := struct {
		JSONRPC string `json:"jsonrpc"`
		ID      any    `json:"id"`
		Error   struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}{
		JSONRPC: "2.0",
		ID:      nil,
		Error: struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{
			Code:    -32603,
			Message: fmt.Sprintf("complypack startup failed: %v", err),
		},
	}
	data, encErr := json.Marshal(resp)
	if encErr != nil {
		return // best-effort; stderr still has the error
	}
	data = append(data, '\n')
	_, _ = os.Stdout.Write(data)
}

// parseSchemaFlags converts --schema flag values into SchemaRef values.
//
//   - "kubernetes"                        -> SchemaRef{Platform: "kubernetes"} (embedded)
//   - "ci=cue://cue.dev/x/actions@v0"    -> SchemaRef{Platform: "ci", Source: "cue://..."}
func parseSchemaFlags(schemas []string) ([]config.SchemaRef, error) {
	if len(schemas) == 0 {
		return nil, nil
	}

	refs := make([]config.SchemaRef, 0, len(schemas))
	for _, s := range schemas {
		if s == "" {
			return nil, fmt.Errorf("empty schema flag value")
		}

		ref := config.SchemaRef{}
		if idx := strings.IndexByte(s, '='); idx >= 0 {
			ref.Platform = s[:idx]
			ref.Source = s[idx+1:]
			if ref.Platform == "" {
				return nil, fmt.Errorf("empty platform name in schema flag %q", s)
			}
			if ref.Source == "" {
				return nil, fmt.Errorf("empty source for platform %q in schema flag %q", ref.Platform, s)
			}
		} else {
			ref.Platform = s
		}
		refs = append(refs, ref)
	}
	return refs, nil
}
