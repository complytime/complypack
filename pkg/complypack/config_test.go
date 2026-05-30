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
	}{
		{
			name: "valid minimal config",
			cfg: complypack.Config{
				EvaluatorID: "io.complytime.opa",
				Version:     "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "valid with provenance",
			cfg: complypack.Config{
				EvaluatorID: "io.complytime.opa",
				Version:     "1.0.0",
				Source: &complypack.Provenance{
					GemaraContent: "oci://registry/gemara/controls:latest",
					PolicyID:      "policy-123",
				},
			},
			wantErr: false,
		},
		{
			name: "missing evaluator-id",
			cfg: complypack.Config{
				Version: "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "missing version",
			cfg: complypack.Config{
				EvaluatorID: "io.complytime.opa",
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
				EvaluatorID: "io.complytime.opa",
				Version:     "1.0.0",
				Source: &complypack.Provenance{
					GemaraContent: "",
					PolicyID:      "policy-123",
				},
			},
			wantErr: true,
		},
		{
			name: "provenance with empty policy-id",
			cfg: complypack.Config{
				EvaluatorID: "io.complytime.opa",
				Version:     "1.0.0",
				Source: &complypack.Provenance{
					GemaraContent: "oci://registry/gemara/controls:latest",
					PolicyID:      "",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigJSON(t *testing.T) {
	cfg := complypack.Config{
		EvaluatorID: "io.complytime.opa",
		Version:     "1.0.0",
		Source: &complypack.Provenance{
			GemaraContent: "oci://registry/gemara/controls:latest",
			PolicyID:      "policy-123",
		},
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	var decoded complypack.Config
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, cfg.EvaluatorID, decoded.EvaluatorID)
	assert.Equal(t, cfg.Version, decoded.Version)
	require.NotNil(t, decoded.Source)
	assert.Equal(t, cfg.Source.GemaraContent, decoded.Source.GemaraContent)
	assert.Equal(t, cfg.Source.PolicyID, decoded.Source.PolicyID)
}

func TestConfigJSONOmitEmpty(t *testing.T) {
	cfg := complypack.Config{
		EvaluatorID: "io.complytime.opa",
		Version:     "1.0.0",
		Source:      nil,
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	// Verify source is omitted when nil
	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	_, exists := raw["source"]
	assert.False(t, exists, "source field should be omitted when nil")
}
