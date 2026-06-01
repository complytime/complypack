package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// CatalogRef represents a reference to a Gemara catalog file.
type CatalogRef struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// SchemaRef represents a reference to a platform schema file.
type SchemaRef struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// ComplyPackConfig represents the structure of complypack.yaml.
type ComplyPackConfig struct {
	Platform        string       `yaml:"platform"`
	GemaraCatalogs  []CatalogRef `yaml:"gemara-catalogs"`
	PlatformSchemas []SchemaRef  `yaml:"platform-schemas"`
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
	if c.Platform == "" {
		return fmt.Errorf("missing required field: platform")
	}

	if len(c.GemaraCatalogs) == 0 {
		return fmt.Errorf("missing required field: gemara-catalogs")
	}

	return nil
}
