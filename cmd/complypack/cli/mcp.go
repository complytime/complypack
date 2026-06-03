// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/complytime/complypack/internal/mcp"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
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

The server runs until interrupted (Ctrl+C) or the client disconnects.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Resolve cache directory
			resolvedCacheDir := cacheDir
			if resolvedCacheDir == "" {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get user home directory: %w", err)
				}
				resolvedCacheDir = filepath.Join(homeDir, ".complypack", "cache")
			}

			// Create MCP server
			opts := &mcp.ServerOptions{
				ConfigPath: configPath,
				CacheDir:   resolvedCacheDir,
			}

			server, err := mcp.NewServer(ctx, opts)
			if err != nil {
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
	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", "Cache directory (default: $HOME/.complypack/cache)")

	return cmd
}
