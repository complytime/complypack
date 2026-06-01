package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SchemaRef represents a platform schema with its path and platform identifier.
type SchemaRef struct {
	Path     string `yaml:"path"`
	Platform string `yaml:"platform"`
}

// GemaraConfig represents Gemara catalog source configuration.
type GemaraConfig struct {
	Source string `yaml:"source"`
}

// ComplyPackConfig represents the structure of complypack.yaml.
// Aligned with CEP-0001 and complypack-pipeline specification.
type ComplyPackConfig struct {
	EvaluatorID string         `yaml:"evaluator-id"`
	Version     string         `yaml:"version"`
	Gemara      GemaraConfig   `yaml:"gemara"`
	Schemas     []SchemaRef    `yaml:"schemas"`
	Policies    *DirConfig     `yaml:"policies,omitempty"`
	Tests       *DirConfig     `yaml:"tests,omitempty"`
	Fixtures    *DirConfig     `yaml:"fixtures,omitempty"`
	Output      *DirConfig     `yaml:"output,omitempty"`
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
	if c.EvaluatorID == "" {
		return fmt.Errorf("missing required field: evaluator-id")
	}

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
		if schema.Path == "" {
			return fmt.Errorf("schema %d missing required field: path", i)
		}
		if schema.Platform == "" {
			return fmt.Errorf("schema %d missing required field: platform", i)
		}
	}

	return nil
}
