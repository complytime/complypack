// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"testing"

	"github.com/complytime/complypack/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMcpCommand(t *testing.T) {
	root := New()

	// Find the mcp command
	mcpCmd, _, err := root.Find([]string{"mcp"})
	require.NoError(t, err, "mcp command should exist")
	assert.Equal(t, "mcp", mcpCmd.Name())
	assert.NotEmpty(t, mcpCmd.Short, "mcp command should have a short description")

	// Find the serve subcommand
	serveCmd, _, err := mcpCmd.Find([]string{"serve"})
	require.NoError(t, err, "mcp serve command should exist")
	assert.Equal(t, "serve", serveCmd.Name())
	assert.NotEmpty(t, serveCmd.Short, "serve command should have a short description")

	// Check flags exist
	flags := serveCmd.Flags()
	assert.NotNil(t, flags.Lookup("config"), "should have --config flag")
	assert.NotNil(t, flags.Lookup("cache-dir"), "should have --cache-dir flag")
	assert.NotNil(t, flags.Lookup("source"), "should have --source flag")
	assert.NotNil(t, flags.Lookup("schema"), "should have --schema flag")
}

func TestParseSourceFlags(t *testing.T) {
	tests := []struct {
		name    string
		sources []string
		want    []config.GemaraSourceEntry
		wantErr string
	}{
		{
			name:    "single OCI source with TLS",
			sources: []string{"oci://ghcr.io/org/catalog:v1"},
			want: []config.GemaraSourceEntry{
				{Source: "oci://ghcr.io/org/catalog:v1", PlainHTTP: false},
			},
		},
		{
			name:    "single OCI source with plain HTTP",
			sources: []string{"oci+http://localhost:5000/catalog:v1"},
			want: []config.GemaraSourceEntry{
				{Source: "oci://localhost:5000/catalog:v1", PlainHTTP: true},
			},
		},
		{
			name: "multiple mixed sources",
			sources: []string{
				"oci://ghcr.io/org/catalog:v1",
				"oci+http://localhost:5000/guidance:latest",
				"oci://ghcr.io/org/policy:v2",
			},
			want: []config.GemaraSourceEntry{
				{Source: "oci://ghcr.io/org/catalog:v1", PlainHTTP: false},
				{Source: "oci://localhost:5000/guidance:latest", PlainHTTP: true},
				{Source: "oci://ghcr.io/org/policy:v2", PlainHTTP: false},
			},
		},
		{
			name:    "empty source",
			sources: []string{""},
			wantErr: "empty source flag value",
		},
		{
			name:    "nil sources returns nil",
			sources: nil,
			want:    nil,
		},
		{
			name:    "empty slice returns nil",
			sources: []string{},
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSourceFlags(tt.sources)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildConfigFromFlags(t *testing.T) {
	tests := []struct {
		name    string
		sources []string
		schemas []string
		want    *config.ComplyPackConfig
		wantErr string
	}{
		{
			name:    "single source and schema",
			sources: []string{"oci://ghcr.io/org/catalog:v1"},
			schemas: []string{"kubernetes"},
			want: &config.ComplyPackConfig{
				Gemara: config.GemaraConfig{
					Sources: []config.GemaraSourceEntry{
						{Source: "oci://ghcr.io/org/catalog:v1", PlainHTTP: false},
					},
				},
				Schemas: []config.SchemaRef{
					{Platform: "kubernetes"},
				},
			},
		},
		{
			name: "multiple sources and schemas",
			sources: []string{
				"oci://ghcr.io/org/catalog:v1",
				"oci+http://localhost:5000/guidance:latest",
			},
			schemas: []string{
				"kubernetes",
				"ci=cue://cue.dev/x/githubactions@v0#Workflow",
			},
			want: &config.ComplyPackConfig{
				Gemara: config.GemaraConfig{
					Sources: []config.GemaraSourceEntry{
						{Source: "oci://ghcr.io/org/catalog:v1", PlainHTTP: false},
						{Source: "oci://localhost:5000/guidance:latest", PlainHTTP: true},
					},
				},
				Schemas: []config.SchemaRef{
					{Platform: "kubernetes"},
					{Platform: "ci", Source: "cue://cue.dev/x/githubactions@v0#Workflow"},
				},
			},
		},
		{
			name:    "empty sources and schemas",
			sources: nil,
			schemas: nil,
			want: &config.ComplyPackConfig{
				Gemara: config.GemaraConfig{
					Sources: nil,
				},
				Schemas: nil,
			},
		},
		{
			name:    "invalid source",
			sources: []string{""},
			schemas: []string{"kubernetes"},
			wantErr: "empty source flag value",
		},
		{
			name:    "invalid schema",
			sources: []string{"oci://ghcr.io/org/catalog:v1"},
			schemas: []string{""},
			wantErr: "empty schema flag value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildConfigFromFlags(tt.sources, tt.schemas)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseSchemaFlags(t *testing.T) {
	tests := []struct {
		name    string
		schemas []string
		want    []config.SchemaRef
		wantErr string
	}{
		{
			name:    "bare platform name uses embedded schema",
			schemas: []string{"kubernetes"},
			want: []config.SchemaRef{
				{Platform: "kubernetes"},
			},
		},
		{
			name:    "platform with external CUE source",
			schemas: []string{"ci=cue://cue.dev/x/githubactions@v0#Workflow"},
			want: []config.SchemaRef{
				{Platform: "ci", Source: "cue://cue.dev/x/githubactions@v0#Workflow"},
			},
		},
		{
			name:    "platform with HTTPS source",
			schemas: []string{"terraform=https://example.com/schema.json"},
			want: []config.SchemaRef{
				{Platform: "terraform", Source: "https://example.com/schema.json"},
			},
		},
		{
			name: "mixed embedded and external schemas",
			schemas: []string{
				"kubernetes",
				"ci=cue://cue.dev/x/githubactions@v0#Workflow",
				"docker",
			},
			want: []config.SchemaRef{
				{Platform: "kubernetes"},
				{Platform: "ci", Source: "cue://cue.dev/x/githubactions@v0#Workflow"},
				{Platform: "docker"},
			},
		},
		{
			name:    "empty schema",
			schemas: []string{""},
			wantErr: "empty schema flag value",
		},
		{
			name:    "empty platform in key=value",
			schemas: []string{"=cue://something"},
			wantErr: "empty platform name",
		},
		{
			name:    "empty source in key=value",
			schemas: []string{"ci="},
			wantErr: "empty source for platform",
		},
		{
			name:    "nil schemas returns nil",
			schemas: nil,
			want:    nil,
		},
		{
			name:    "empty slice returns nil",
			schemas: []string{},
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSchemaFlags(tt.schemas)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
