// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"testing"

	"github.com/complytime/complypack/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			name:    "schema only without sources",
			sources: nil,
			schemas: []string{"kubernetes"},
			want: &config.ComplyPackConfig{
				Gemara: config.GemaraConfig{
					Sources: nil,
				},
				Schemas: []config.SchemaRef{
					{Platform: "kubernetes"},
				},
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

func TestBuildSchemaRefs_WithFlags(t *testing.T) {
	refs, err := buildSchemaRefs(
		"kubernetes-deployment",
		[]string{
			"kubernetes-deployment=" +
				"cue://example.com/schema",
		},
	)
	require.NoError(t, err)
	assert.Len(t, refs, 1)
	assert.Equal(t,
		"kubernetes-deployment", refs[0].Platform)
	assert.Equal(t,
		"cue://example.com/schema", refs[0].Source)
}

func TestBuildSchemaRefs_WithoutFlags(t *testing.T) {
	refs, err := buildSchemaRefs(
		"kubernetes-deployment", nil,
	)
	require.NoError(t, err)
	assert.Len(t, refs, 1)
	assert.Equal(t,
		"kubernetes-deployment", refs[0].Platform)
	assert.Empty(t, refs[0].Source,
		"should rely on embedded index")
}
