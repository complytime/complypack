// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSchemaSource(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantType    SchemaSourceType
		wantPath    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "cue module",
			source:   "cue://cue.dev/x/k8s.io/api/core/v1",
			wantType: SourceTypeCUEModule,
			wantPath: "cue.dev/x/k8s.io/api/core/v1",
		},
		{
			name:     "https URL",
			source:   "https://example.com/schemas/terraform.json",
			wantType: SourceTypeHTTPS,
			wantPath: "https://example.com/schemas/terraform.json",
		},
		{
			name:     "http URL",
			source:   "http://localhost:8080/schema.cue",
			wantType: SourceTypeHTTP,
			wantPath: "http://localhost:8080/schema.cue",
		},
		{
			name:     "file path absolute",
			source:   "file:///etc/schemas/docker.cue",
			wantType: SourceTypeFile,
			wantPath: "/etc/schemas/docker.cue",
		},
		{
			name:     "file path relative",
			source:   "file://./schemas/custom.json",
			wantType: SourceTypeFile,
			wantPath: "./schemas/custom.json",
		},
		{
			name:     "legacy path",
			source:   "./schemas/old-style.cue",
			wantType: SourceTypeLegacyPath,
			wantPath: "./schemas/old-style.cue",
		},
		{
			name:     "empty source",
			source:   "",
			wantType: SourceTypeUnknown,
			wantPath: "",
		},
		{
			name:        "cue scheme without path",
			source:      "cue://",
			wantErr:     true,
			errContains: "requires module path",
		},
		{
			name:        "file scheme without path",
			source:      "file://",
			wantErr:     true,
			errContains: "requires path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseSchemaSource(tt.source)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantType, result.Type)
			assert.Equal(t, tt.wantPath, result.Path)
		})
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name string
		path string
		want SchemaFormat
	}{
		{
			name: "json extension",
			path: "schema.json",
			want: FormatJSON,
		},
		{
			name: "cue extension",
			path: "schema.cue",
			want: FormatCUE,
		},
		{
			name: "json with path",
			path: "/path/to/schema.json",
			want: FormatJSON,
		},
		{
			name: "cue with path",
			path: "./schemas/kubernetes.cue",
			want: FormatCUE,
		},
		{
			name: "no extension",
			path: "schema",
			want: FormatUnknown,
		},
		{
			name: "wrong extension",
			path: "schema.txt",
			want: FormatUnknown,
		},
		{
			name: "URL with json",
			path: "https://example.com/schema.json",
			want: FormatJSON,
		},
		{
			name: "URL with cue",
			path: "https://example.com/schema.cue",
			want: FormatCUE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFormat(tt.path)
			assert.Equal(t, tt.want, result)
		})
	}
}
