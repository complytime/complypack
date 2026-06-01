// SPDX-License-Identifier: Apache-2.0

package schemas

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedSchemas(t *testing.T) {
	platforms := []string{"kubernetes", "terraform", "docker", "ansible", "ci"}

	for _, platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			data, err := JSONSchemas.ReadFile("json-schema/" + platform + ".json")
			require.NoError(t, err, "should read embedded schema")
			assert.NotEmpty(t, data, "schema should not be empty")
			assert.Contains(t, string(data), `"components"`, "should be OpenAPI format")
		})
	}
}

func TestGetBuiltInSchema(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		wantErr  bool
	}{
		{"kubernetes exists", "kubernetes", false},
		{"terraform exists", "terraform", false},
		{"unknown platform", "foobar", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := GetBuiltInSchema(tt.platform)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, data)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, data)
			}
		})
	}
}
