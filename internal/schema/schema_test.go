// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"context"
	"testing"

	"cuelang.org/go/cue"

	"github.com/complytime/complypack/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadTestCUESchema(
	t *testing.T, platform string,
) cue.Value {
	t.Helper()
	ctx := context.Background()
	refs := []config.SchemaRef{
		{Platform: platform},
	}
	_, cueSchemas, err := LoadFromConfig(
		ctx, refs, DefaultRegistry(),
	)
	require.NoError(t, err)

	cueSchema, ok := cueSchemas[platform]
	require.True(t, ok,
		"schema should be loaded for platform %s",
		platform)
	return cueSchema
}

func TestValidateData_Valid(t *testing.T) {
	cueSchema := loadTestCUESchema(t, "kubernetes-pod")

	testData := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "test",
					"image": "nginx:latest",
				},
			},
		},
	}

	errs := ValidateData(testData, cueSchema)
	assert.Empty(t, errs)
}

func TestValidateData_Invalid(t *testing.T) {
	cueSchema := loadTestCUESchema(t, "kubernetes-pod")

	testData := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
		},
		"spec": map[string]interface{}{
			"containers": "not-a-list",
		},
	}

	errs := ValidateData(testData, cueSchema)
	assert.NotEmpty(t, errs,
		"should report validation errors")
}
