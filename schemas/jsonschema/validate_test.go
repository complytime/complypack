// SPDX-License-Identifier: Apache-2.0

package jsonschema

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateConfig_ValidFullConfig(t *testing.T) {
	yaml := []byte(`
id: io.complytime.test
evaluator-id: opa
version: 1.0.0
gemara:
  sources:
    - source: oci://ghcr.io/org/catalog:v1
schemas:
  - platform: kubernetes-deployment
`)
	warnings, err := ValidateConfig(yaml, false)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateConfig_ValidMinimalConfig(t *testing.T) {
	yaml := []byte(`
version: 0.1.0
`)
	warnings, err := ValidateConfig(yaml, false)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateConfig_EmptyDocument(t *testing.T) {
	yaml := []byte(`{}`)
	warnings, err := ValidateConfig(yaml, false)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateConfig_MalformedYAML(t *testing.T) {
	yaml := []byte(`[invalid: yaml: :`)
	_, err := ValidateConfig(yaml, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing YAML")
}

func TestValidateConfig_NullDocument(t *testing.T) {
	warnings, err := ValidateConfig([]byte(""), false)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateConfig_InvalidID_Spaces(t *testing.T) {
	yaml := []byte(`
id: "bob bobby"
version: 0.1.0
`)
	_, err := ValidateConfig(yaml, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reverse-domain notation")
}

func TestValidateConfig_InvalidID_Uppercase(t *testing.T) {
	yaml := []byte(`
id: IO.Complytime.Test
version: 0.1.0
`)
	_, err := ValidateConfig(yaml, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestValidateConfig_InvalidID_SingleSegment(t *testing.T) {
	yaml := []byte(`
id: mypack
version: 0.1.0
`)
	_, err := ValidateConfig(yaml, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestValidateConfig_ValidID_TwoSegments(t *testing.T) {
	yaml := []byte(`
id: io.test
version: 0.1.0
`)
	warnings, err := ValidateConfig(yaml, false)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateConfig_InvalidVersion_NotSemver(t *testing.T) {
	yaml := []byte(`
version: latest
`)
	_, err := ValidateConfig(yaml, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be semver")
}

func TestValidateConfig_ValidVersion_PreRelease(t *testing.T) {
	yaml := []byte(`
version: 1.0.0-alpha.1
`)
	warnings, err := ValidateConfig(yaml, false)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateConfig_ValidVersion_PreReleaseWithHyphen(t *testing.T) {
	yaml := []byte(`
version: 1.0.0-rc-1
`)
	warnings, err := ValidateConfig(yaml, false)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateConfig_ValidVersion_BuildMetadata(t *testing.T) {
	yaml := []byte(`
version: 1.0.0+build.123
`)
	warnings, err := ValidateConfig(yaml, false)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateConfig_InvalidEvaluatorID_Spaces(t *testing.T) {
	yaml := []byte(`
evaluator-id: "my evaluator"
version: 0.1.0
`)
	_, err := ValidateConfig(yaml, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lowercase identifier")
}

func TestValidateConfig_ValidEvaluatorID_WithDots(t *testing.T) {
	yaml := []byte(`
evaluator-id: io.complytime.opa
version: 0.1.0
`)
	warnings, err := ValidateConfig(yaml, false)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateConfig_ValidEvaluatorID_WithUnderscores(t *testing.T) {
	yaml := []byte(`
evaluator-id: my_evaluator
version: 0.1.0
`)
	warnings, err := ValidateConfig(yaml, false)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateConfig_GemaraLegacySingleSource(t *testing.T) {
	yaml := []byte(`
version: 0.1.0
gemara:
  source: catalogs/controls.yaml
  plain-http: true
`)
	warnings, err := ValidateConfig(yaml, false)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateConfig_GemaraMultiSource(t *testing.T) {
	yaml := []byte(`
version: 0.1.0
gemara:
  sources:
    - source: catalogs/controls.yaml
    - source: ghcr.io/org/guidance:v1
      plain-http: true
`)
	warnings, err := ValidateConfig(yaml, false)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateConfig_GemaraSourceEmpty(t *testing.T) {
	yaml := []byte(`
version: 0.1.0
gemara:
  sources:
    - source: ""
`)
	_, err := ValidateConfig(yaml, false)
	require.Error(t, err)
}

func TestValidateConfig_SchemasMissingPlatform(t *testing.T) {
	yaml := []byte(`
version: 0.1.0
schemas:
  - source: cue://example.com/schema
`)
	_, err := ValidateConfig(yaml, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "platform")
}

func TestValidateConfig_SchemasWithDeprecatedPath(t *testing.T) {
	yaml := []byte(`
version: 0.1.0
schemas:
  - platform: kubernetes
    path: ./schemas/k8s.cue
`)
	warnings, err := ValidateConfig(yaml, false)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateConfig_DirConfigValid(t *testing.T) {
	yaml := []byte(`
version: 0.1.0
policies:
  dir: policies/
  helpers:
    - helpers/common.rego
tests:
  dir: tests/
fixtures:
  dir: fixtures/
output:
  dir: out/
`)
	warnings, err := ValidateConfig(yaml, false)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateConfig_DirConfigMissingDir(t *testing.T) {
	yaml := []byte(`
version: 0.1.0
policies:
  helpers:
    - helpers/common.rego
`)
	_, err := ValidateConfig(yaml, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dir")
}

func TestConvertYAMLToJSON_MapAnyAny(t *testing.T) {
	// Older YAML libraries decode mappings as map[any]any
	input := map[any]any{
		"name":    "test",
		"count":   42,
		"enabled": true,
	}
	result := convertYAMLToJSON(input)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test", m["name"])
	assert.Equal(t, float64(42), m["count"])
	assert.Equal(t, true, m["enabled"])
}

func TestConvertYAMLToJSON_Int64(t *testing.T) {
	var val int64 = 9223372036854775807
	result := convertYAMLToJSON(val)
	f, ok := result.(float64)
	require.True(t, ok)
	assert.Equal(t, float64(val), f)
}

func TestConvertYAMLToJSON_Int(t *testing.T) {
	result := convertYAMLToJSON(42)
	f, ok := result.(float64)
	require.True(t, ok)
	assert.Equal(t, float64(42), f)
}

func TestConvertYAMLToJSON_Nil(t *testing.T) {
	result := convertYAMLToJSON(nil)
	assert.Nil(t, result)
}

func TestConvertYAMLToJSON_NestedMapAnyAny(t *testing.T) {
	input := map[any]any{
		"outer": map[any]any{
			"inner": "value",
		},
		"list": []any{"a", "b"},
	}
	result := convertYAMLToJSON(input)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	inner, ok := m["outer"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "value", inner["inner"])
	list, ok := m["list"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"a", "b"}, list)
}

func TestConvertYAMLToJSON_Slice(t *testing.T) {
	input := []any{1, "two", map[string]any{"k": "v"}}
	result := convertYAMLToJSON(input)
	s, ok := result.([]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), s[0])
	assert.Equal(t, "two", s[1])
	inner, ok := s[2].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "v", inner["k"])
}

func TestValidateConfig_IntegerVersion(t *testing.T) {
	yaml := []byte(`
version: 1
`)
	_, err := ValidateConfig(yaml, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version must be a string, not a number")
	assert.NotContains(t, err.Error(), "&{")
}

func TestValidateConfig_IntegerID(t *testing.T) {
	yaml := []byte(`
id: 123
version: 1.0.0
`)
	_, err := ValidateConfig(yaml, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id must be a string")
	assert.NotContains(t, err.Error(), "&{")
}

func TestValidateConfig_FallbackErrorUsesLocalizedString(t *testing.T) {
	// Trigger a validation error without a friendlyErrors mapping.
	// gemara.sources[0].source with minLength=1 violated by empty string
	// produces a minLength error that has no friendly mapping, so exercises
	// the LocalizedString fallback.
	yaml := []byte(`
version: 0.1.0
gemara:
  sources:
    - source: ""
`)
	_, err := ValidateConfig(yaml, false)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "&{", "error should not contain raw Go struct")
}

func TestFormatValidationError_NonValidationError(t *testing.T) {
	err := fmt.Errorf("some other error")
	result := formatValidationError(err)
	assert.Equal(t, err, result)
}
