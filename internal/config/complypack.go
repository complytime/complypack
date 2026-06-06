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

// GemaraSourceEntry represents a single Gemara artifact source.
type GemaraSourceEntry struct {
	Source    string `yaml:"source"`
	PlainHTTP bool   `yaml:"plain-http,omitempty"`
}

// GemaraConfig represents Gemara catalog source configuration.
// Supports both legacy single-source format and multi-source format:
//
//	# Legacy (still supported):
//	gemara:
//	  source: catalogs/controls.yaml
//
//	# Multi-source:
//	gemara:
//	  sources:
//	    - source: catalogs/controls.yaml
//	    - source: ghcr.io/org/guidance:v1
//	      plain-http: true
type GemaraConfig struct {
	Sources []GemaraSourceEntry
}

func (g *GemaraConfig) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		Source    string             `yaml:"source"`
		Sources  []GemaraSourceEntry `yaml:"sources"`
		PlainHTTP bool              `yaml:"plain-http,omitempty"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}

	if raw.Source != "" && len(raw.Sources) > 0 {
		return fmt.Errorf("gemara config: cannot specify both 'source' and 'sources'; use 'sources' for multiple entries")
	}

	if raw.Source != "" {
		g.Sources = []GemaraSourceEntry{{Source: raw.Source, PlainHTTP: raw.PlainHTTP}}
	} else {
		g.Sources = raw.Sources
	}

	return nil
}

// ComplyPackConfig represents the structure of complypack.yaml.
// Aligned with CEP-0001 and complypack-pipeline specification.
type ComplyPackConfig struct {
	ID          string       `yaml:"id"`
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

	// Validate schema entries if present
	for i, schema := range c.Schemas {
		if schema.Platform == "" {
			return fmt.Errorf("schema %d missing required field: platform", i)
		}
	}

	return nil
}

// ValidateForMCP checks fields required for MCP server operation.
func (c *ComplyPackConfig) ValidateForMCP() error {
	if err := c.Validate(); err != nil {
		return err
	}

	if len(c.Gemara.Sources) == 0 {
		return fmt.Errorf("missing required field: gemara.sources (at least one source required)")
	}

	if len(c.Schemas) == 0 {
		return fmt.Errorf("missing required field: schemas")
	}

	return nil
}

// ValidateForPack checks fields required for pack operation.
func (c *ComplyPackConfig) ValidateForPack() error {
	if err := c.Validate(); err != nil {
		return err
	}

	if c.ID == "" {
		return fmt.Errorf("missing required field: id")
	}

	if c.EvaluatorID == "" {
		return fmt.Errorf("missing required field: evaluator-id")
	}

	return nil
}
