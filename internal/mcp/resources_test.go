// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceStore_ListResources(t *testing.T) {
	catalogs := map[string][]byte{
		"controls-v1": []byte("catalog: controls-v1\ncontrols: []"),
		"security-v2": []byte("catalog: security-v2\ncontrols: []"),
	}

	schemas := map[string][]byte{
		"kubernetes": []byte(`{"components": {}}`),
		"terraform":  []byte(`{"components": {}}`),
	}

	store := NewResourceStore(catalogs, nil, nil, nil, schemas, nil)
	resources, err := store.ListResources(context.Background())
	require.NoError(t, err)

	assert.Len(t, resources, 4, "should have 2 catalogs + 2 schemas")

	// Check catalog URIs
	catalogURIs := []string{}
	for _, r := range resources {
		if r.MIMEType == MIMETypeYAML {
			catalogURIs = append(catalogURIs, r.URI)
		}
	}
	assert.Contains(t, catalogURIs, "complypack://catalog/controls-v1")
	assert.Contains(t, catalogURIs, "complypack://catalog/security-v2")

	// Check schema URIs
	schemaURIs := []string{}
	for _, r := range resources {
		if r.MIMEType == MIMETypeJSONSchema {
			schemaURIs = append(schemaURIs, r.URI)
		}
	}
	assert.Contains(t, schemaURIs, "complypack://schema/kubernetes")
	assert.Contains(t, schemaURIs, "complypack://schema/terraform")
}

func TestResourceStore_ReadResource(t *testing.T) {
	catalogs := map[string][]byte{
		"controls-v1": []byte("catalog: controls-v1\ncontrols: []"),
	}

	schemas := map[string][]byte{
		"kubernetes": []byte(`{"components": {}}`),
	}

	store := NewResourceStore(catalogs, nil, nil, nil, schemas, nil)

	tests := []struct {
		name     string
		uri      string
		wantData string
		wantMIME string
		wantErr  bool
	}{
		{
			name:     "read catalog",
			uri:      "complypack://catalog/controls-v1",
			wantData: "catalog: controls-v1\ncontrols: []",
			wantMIME: MIMETypeYAML,
		},
		{
			name:     "read schema",
			uri:      "complypack://schema/kubernetes",
			wantData: `{"components": {}}`,
			wantMIME: MIMETypeJSONSchema,
		},
		{
			name:    "unknown catalog",
			uri:     "complypack://catalog/unknown",
			wantErr: true,
		},
		{
			name:    "unknown schema",
			uri:     "complypack://schema/unknown",
			wantErr: true,
		},
		{
			name:    "invalid URI",
			uri:     "invalid://foo/bar",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents, err := store.ReadResource(context.Background(), tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, contents, 1, "should return exactly one content item")
			assert.Equal(t, tt.wantData, contents[0].Text)
			assert.Equal(t, tt.wantMIME, contents[0].MIMEType)
		})
	}
}
