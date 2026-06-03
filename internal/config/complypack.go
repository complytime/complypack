package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SchemaRef represents a platform schema with its source and platform identifier.
type SchemaRef struct {
	// Platform identifies the target platform (e.g., "kubernetes", "terraform")
	Platform string `yaml:"platform"`

	// Source is a URI specifying where to load the schema from.
	// Supported schemes:
	//   - cue://module.path          -> CUE registry module
	//   - https://example.com/s.json -> HTTP(S) download
	//   - file://./path/to/file      -> Local file
	// If empty, falls back to embedded schemas.
	Source string `yaml:"source,omitempty"`

	// Path is deprecated - use Source with file:// scheme instead.
	// Kept for backward compatibility.
	Path string `yaml:"path,omitempty"`
}

// GemaraConfig represents Gemara catalog source configuration.
type GemaraConfig struct {
	Source    string `yaml:"source"`
	PlainHTTP bool   `yaml:"plain-http,omitempty"` // Use HTTP instead of HTTPS for OCI registries
}

// ComplyPackConfig represents the structure of complypack.yaml.
// Aligned with CEP-0001 and complypack-pipeline specification.
type ComplyPackConfig struct {
	EvaluatorID string       `yaml:"evaluator-id"`
	Version     string       `yaml:"version"`
	Gemara      GemaraConfig `yaml:"gemara"`
	Schemas     []SchemaRef  `yaml:"schemas"`
	Policies    *DirConfig   `yaml:"policies,omitempty"`
	Tests       *DirConfig   `yaml:"tests,omitempty"`
	Fixtures    *DirConfig   `yaml:"fixtures,omitempty"`
	Output      *DirConfig   `yaml:"output,omitempty"`
}

// DirConfig represents a directory configuration.
type DirConfig struct {
	Dir     string   `yaml:"dir"`
	Helpers []string `yaml:"helpers,omitempty"`
}

// LoadConfig reads and parses a complypack.yaml file.
func LoadConfig(path string) (*ComplyPackConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ComplyPackConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

// Validate checks that required fields are present.
func (c *ComplyPackConfig) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("missing required field: version")
	}

	if c.Gemara.Source == "" {
		return fmt.Errorf("missing required field: gemara.source")
	}

	if len(c.Schemas) == 0 {
		return fmt.Errorf("missing required field: schemas")
	}

	// Validate each schema has required fields
	for i, schema := range c.Schemas {
		if schema.Platform == "" {
			return fmt.Errorf("schema %d missing required field: platform", i)
		}
		// Either source or path must be specified (path is legacy)
		// If neither is specified, we'll fall back to embedded schemas
	}

	return nil
}
