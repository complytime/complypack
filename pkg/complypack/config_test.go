// SPDX-License-Identifier: Apache-2.0

package complypack_test

import (
	"encoding/json"
	"testing"

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
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
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
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded complypack.Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.EvaluatorID != cfg.EvaluatorID {
		t.Errorf("EvaluatorID = %q, want %q", decoded.EvaluatorID, cfg.EvaluatorID)
	}
	if decoded.Version != cfg.Version {
		t.Errorf("Version = %q, want %q", decoded.Version, cfg.Version)
	}
	if decoded.Source == nil {
		t.Fatal("Source is nil after unmarshal")
	}
	if decoded.Source.GemaraContent != cfg.Source.GemaraContent {
		t.Errorf("GemaraContent = %q, want %q", decoded.Source.GemaraContent, cfg.Source.GemaraContent)
	}
}

func TestConfigJSONOmitEmpty(t *testing.T) {
	cfg := complypack.Config{
		EvaluatorID: "io.complytime.opa",
		Version:     "1.0.0",
		Source:      nil,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Verify source is omitted when nil
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map error = %v", err)
	}

	if _, exists := raw["source"]; exists {
		t.Error("source field should be omitted when nil")
	}
}
