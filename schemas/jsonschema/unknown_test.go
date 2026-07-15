// SPDX-License-Identifier: Apache-2.0

package jsonschema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckUnknownKeys_NoUnknown(t *testing.T) {
	m := map[string]any{
		"id":           "io.test.pack",
		"evaluator-id": "opa",
		"version":      "1.0.0",
	}
	warnings := checkUnknownKeys(m)
	assert.Empty(t, warnings)
}

func TestCheckUnknownKeys_UnknownTopLevel(t *testing.T) {
	m := map[string]any{
		"version":  "1.0.0",
		"unknown1": "value",
	}
	warnings := checkUnknownKeys(m)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "unknown1")
}

func TestCheckUnknownKeys_TypoWithSuggestion(t *testing.T) {
	m := map[string]any{
		"version":      "1.0.0",
		"evalutaor-id": "opa",
	}
	warnings := checkUnknownKeys(m)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "evalutaor-id")
	assert.Contains(t, warnings[0], "did you mean")
	assert.Contains(t, warnings[0], "evaluator-id")
}

func TestCheckUnknownKeys_TypoNoSuggestion(t *testing.T) {
	m := map[string]any{
		"version":                    "1.0.0",
		"completely-unrelated-field": "value",
	}
	warnings := checkUnknownKeys(m)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "completely-unrelated-field")
	assert.NotContains(t, warnings[0], "did you mean")
}

func TestCheckUnknownKeys_NestedUnknown(t *testing.T) {
	m := map[string]any{
		"version": "1.0.0",
		"gemara": map[string]any{
			"sources": []any{
				map[string]any{
					"source":   "oci://example.com/cat:v1",
					"typo-key": "value",
				},
			},
		},
	}
	warnings := checkUnknownKeys(m)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "typo-key")
}

func TestCheckUnknownKeys_AllKnownFields(t *testing.T) {
	m := map[string]any{
		"id":           "io.test.pack",
		"evaluator-id": "opa",
		"version":      "1.0.0",
		"gemara": map[string]any{
			"sources": []any{
				map[string]any{
					"source":     "oci://example.com/cat:v1",
					"plain-http": true,
				},
			},
		},
		"schemas": []any{
			map[string]any{
				"platform": "kubernetes-deployment",
				"source":   "cue://example.com/schema",
				"path":     "./old-path.cue",
			},
		},
		"policies": map[string]any{
			"dir":     "policies/",
			"helpers": []any{"helpers/common.rego"},
		},
		"tests":    map[string]any{"dir": "tests/"},
		"fixtures": map[string]any{"dir": "fixtures/"},
		"output":   map[string]any{"dir": "out/"},
	}
	warnings := checkUnknownKeys(m)
	assert.Empty(t, warnings)
}

func TestCheckUnknownKeys_UnknownNestedScope(t *testing.T) {
	// When a nested map appears under a scope not registered in knownKeys,
	// walkUnknown silently returns (no warnings for keys in that scope).
	m := map[string]any{
		"version": "1.0.0",
		"output": map[string]any{
			"dir": map[string]any{
				"nested-key": "value",
			},
		},
	}
	// "output.dir" is not a registered scope, so walkUnknown returns without
	// checking keys. The "dir" key IS known under "output", so no warning
	// for "dir" itself. "nested-key" is unreachable.
	warnings := checkUnknownKeys(m)
	assert.Empty(t, warnings)
}

func TestLevenshtein(t *testing.T) {
	assert.Equal(t, 0, levenshtein("abc", "abc"))
	assert.Equal(t, 1, levenshtein("abc", "ab"))
	assert.Equal(t, 1, levenshtein("abc", "adc"))
	assert.Equal(t, 2, levenshtein("evalutaor-id", "evaluator-id"))
	assert.Equal(t, 3, levenshtein("abc", "def"))
	assert.Equal(t, 3, levenshtein("", "abc"))
	assert.Equal(t, 3, levenshtein("abc", ""))
	assert.Equal(t, 0, levenshtein("", ""))
}

func TestValidateConfig_StrictRejectsUnknownKeys(t *testing.T) {
	yaml := []byte(`
version: 0.1.0
evalutaor-id: opa
`)
	warnings, err := ValidateConfig(yaml, false)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "evalutaor-id")

	_, err = ValidateConfig(yaml, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "strict mode")
}
