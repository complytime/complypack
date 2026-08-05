// SPDX-License-Identifier: Apache-2.0

package complypack_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complypack/pkg/complypack"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     complypack.Config
		wantErr bool
		// wantErrContains, when set on a wantErr case, asserts the error
		// message names the specific field/index that triggered it, proving
		// the intended validation branch fired rather than an earlier one.
		wantErrContains string
	}{
		{
			name: "valid minimal config",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "opa",
				Version:     "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "valid with provenance",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "opa",
				Version:     "1.0.0",
				Source: []complypack.Provenance{
					{
						PolicyID: "policy-123",
						GemaraContent: []complypack.GemaraRef{
							{
								ReferenceID: "ref-1",
								URI:         "oci://registry/gemara/controls:latest",
								Version:     "1.0.0",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with url-less gemara ref",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "opa",
				Version:     "1.0.0",
				Source: []complypack.Provenance{
					{
						PolicyID: "policy-123",
						GemaraContent: []complypack.GemaraRef{
							{ReferenceID: "ref-local"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing id",
			cfg: complypack.Config{
				EvaluatorID: "opa",
				Version:     "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "missing evaluator-id",
			cfg: complypack.Config{
				ID:      "io.test.pack",
				Version: "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "missing version",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "opa",
			},
			wantErr: true,
		},
		{
			name:    "empty config",
			cfg:     complypack.Config{},
			wantErr: true,
		},
		{
			name: "provenance with empty gemara-content",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "opa",
				Version:     "1.0.0",
				Source: []complypack.Provenance{
					{
						PolicyID:      "policy-123",
						GemaraContent: nil,
					},
				},
			},
			wantErr:         true,
			wantErrContains: "source[0].gemara-content",
		},
		{
			name: "provenance with empty policy-id",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "opa",
				Version:     "1.0.0",
				Source: []complypack.Provenance{
					{
						PolicyID: "",
						GemaraContent: []complypack.GemaraRef{
							{ReferenceID: "ref-1"},
						},
					},
				},
			},
			wantErr:         true,
			wantErrContains: "source[0].policy-id",
		},
		{
			name: "gemara ref with empty reference-id",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "opa",
				Version:     "1.0.0",
				Source: []complypack.Provenance{
					{
						PolicyID: "policy-123",
						GemaraContent: []complypack.GemaraRef{
							{ReferenceID: "", URI: "oci://registry/x:latest"},
						},
					},
				},
			},
			wantErr:         true,
			wantErrContains: "source[0].gemara-content[0].reference-id",
		},
		{
			name: "second provenance entry with empty policy-id is rejected",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "opa",
				Version:     "1.0.0",
				Source: []complypack.Provenance{
					{
						PolicyID: "policy-1",
						GemaraContent: []complypack.GemaraRef{
							{ReferenceID: "ref-1"},
						},
					},
					{
						PolicyID: "",
						GemaraContent: []complypack.GemaraRef{
							{ReferenceID: "ref-2"},
						},
					},
				},
			},
			wantErr:         true,
			wantErrContains: "source[1].policy-id",
		},
		{
			name: "later gemara ref with empty reference-id is rejected",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "opa",
				Version:     "1.0.0",
				Source: []complypack.Provenance{
					{
						PolicyID: "policy-1",
						GemaraContent: []complypack.GemaraRef{
							{ReferenceID: "ref-ok"},
							{ReferenceID: ""},
						},
					},
				},
			},
			wantErr:         true,
			wantErrContains: "source[0].gemara-content[1].reference-id",
		},
		{
			name: "multiple valid provenance entries accepted",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "opa",
				Version:     "1.0.0",
				Source: []complypack.Provenance{
					{
						PolicyID: "policy-1",
						GemaraContent: []complypack.GemaraRef{
							{ReferenceID: "ref-1"},
						},
					},
					{
						PolicyID: "policy-2",
						GemaraContent: []complypack.GemaraRef{
							{ReferenceID: "ref-2"},
							{ReferenceID: "ref-3"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid id format - single segment",
			cfg: complypack.Config{
				ID:          "notsegmented",
				EvaluatorID: "opa",
				Version:     "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "invalid id format - uppercase",
			cfg: complypack.Config{
				ID:          "IO.Test.Pack",
				EvaluatorID: "opa",
				Version:     "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "invalid evaluator-id format - spaces",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "my bad evaluator",
				Version:     "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "invalid evaluator-id format - uppercase",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "OPA",
				Version:     "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "invalid version format - not semver",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "opa",
				Version:     "latest",
			},
			wantErr: true,
		},
		{
			name: "invalid version format - coerced integer string",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "opa",
				Version:     "1",
			},
			wantErr: true,
		},
		{
			name: "valid evaluator-id with dots",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "io.custom.eval",
				Version:     "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "valid version with prerelease",
			cfg: complypack.Config{
				ID:          "io.test.pack",
				EvaluatorID: "opa",
				Version:     "1.0.0-rc.1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, complypack.ErrInvalidConfig)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains,
						"error should name the field/index that triggered it")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigJSON(t *testing.T) {
	cfg := complypack.Config{
		ID:          "io.test.pack",
		EvaluatorID: "opa",
		Version:     "1.0.0",
		Source: []complypack.Provenance{
			{
				PolicyID: "policy-123",
				GemaraContent: []complypack.GemaraRef{
					{
						ReferenceID: "ref-1",
						URI:         "oci://registry/gemara/controls:latest",
						Version:     "1.0.0",
					},
				},
			},
		},
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	var decoded complypack.Config
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, cfg.ID, decoded.ID)
	assert.Equal(t, cfg.EvaluatorID, decoded.EvaluatorID)
	assert.Equal(t, cfg.Version, decoded.Version)
	require.Len(t, decoded.Source, 1)
	assert.Equal(t, cfg.Source[0].PolicyID, decoded.Source[0].PolicyID)
	require.Len(t, decoded.Source[0].GemaraContent, 1)
	assert.Equal(t, cfg.Source[0].GemaraContent[0].ReferenceID, decoded.Source[0].GemaraContent[0].ReferenceID)
	assert.Equal(t, cfg.Source[0].GemaraContent[0].URI, decoded.Source[0].GemaraContent[0].URI)
	assert.Equal(t, cfg.Source[0].GemaraContent[0].Version, decoded.Source[0].GemaraContent[0].Version)
}

func TestConfigJSONFieldNames(t *testing.T) {
	cfg := complypack.Config{
		ID:          "io.test.pack",
		EvaluatorID: "opa",
		Version:     "1.0.0",
		Source: []complypack.Provenance{
			{
				PolicyID: "policy-123",
				GemaraContent: []complypack.GemaraRef{
					{ReferenceID: "ref-1", URI: "oci://registry/x:latest", Version: "1.0.0"},
				},
			},
		},
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	source, ok := raw["source"].([]any)
	require.True(t, ok, "source should marshal as an array")
	require.Len(t, source, 1)

	entry, ok := source[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "policy-123", entry["policy-id"])

	content, ok := entry["gemara-content"].([]any)
	require.True(t, ok, "gemara-content should marshal as an array")
	require.Len(t, content, 1)

	ref, ok := content[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ref-1", ref["reference-id"])
	assert.Equal(t, "oci://registry/x:latest", ref["uri"])
	assert.Equal(t, "1.0.0", ref["version"])
}

func TestConfigJSONOmitEmpty(t *testing.T) {
	cfg := complypack.Config{
		ID:          "io.test.pack",
		EvaluatorID: "opa",
		Version:     "1.0.0",
		Source:      nil,
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	_, exists := raw["source"]
	assert.False(t, exists, "source field should be omitted when nil")
}

func TestGemaraRefJSONOmitEmpty(t *testing.T) {
	ref := complypack.GemaraRef{ReferenceID: "ref-local"}

	data, err := json.Marshal(ref)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.Equal(t, "ref-local", raw["reference-id"])
	_, hasURI := raw["uri"]
	assert.False(t, hasURI, "uri should be omitted when empty")
	_, hasVersion := raw["version"]
	assert.False(t, hasVersion, "version should be omitted when empty")
}
