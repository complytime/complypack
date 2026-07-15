// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	"github.com/complytime/complypack/schemas/jsonschema"
	"gopkg.in/yaml.v3"
)

// SchemaRef represents a platform schema with its source and platform identifier.
type SchemaRef struct {
	// Platform identifies the target platform (e.g., "kubernetes-deployment", "ci-github-actions")
	Platform string `yaml:"platform"`

	// Source is a URI specifying where to load the schema from.
	// Supported schemes:
	//   - cue://module.path          -> CUE registry module
	//   - https://example.com/s.json -> HTTP(S) download
	//   - file://./path/to/file      -> Local file
	// If empty, falls back to index defaults.
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
		Source    string              `yaml:"source"`
		Sources   []GemaraSourceEntry `yaml:"sources"`
		PlainHTTP bool                `yaml:"plain-http,omitempty"`
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
	ID          string       `yaml:"id,omitempty"`
	EvaluatorID string       `yaml:"evaluator-id,omitempty"`
	Version     string       `yaml:"version,omitempty"`
	Gemara      GemaraConfig `yaml:"gemara,omitempty"`
	Schemas     []SchemaRef  `yaml:"schemas,omitempty"`
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
// Validation warnings are written to w. Pass os.Stderr for default behavior.
func LoadConfig(path string, strict bool, w io.Writer) (*ComplyPackConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	warnings, err := jsonschema.ValidateConfig(data, strict)
	if err != nil {
		return nil, fmt.Errorf("config schema validation: %w", err)
	}
	for _, warning := range warnings {
		fmt.Fprintf(w, "WARNING: %s\n", warning)
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

// validateFormats checks format constraints on any non-empty identity fields.
// Called by all scope-specific validators so format rules are enforced
// consistently regardless of entry point. Patterns come from the embedded
// JSON Schema — single source of truth.
func (c *ComplyPackConfig) validateFormats() error {
	if c.ID != "" {
		if err := validateFormat("id", c.ID, jsonschema.IDPattern()); err != nil {
			return err
		}
	}
	if c.EvaluatorID != "" {
		if err := validateFormat("evaluator-id", c.EvaluatorID, jsonschema.EvaluatorIDPattern()); err != nil {
			return err
		}
	}
	if c.Version != "" {
		if err := validateFormat("version", c.Version, jsonschema.VersionPattern()); err != nil {
			return err
		}
	}
	return nil
}

// validateFormat checks a field value against its JSON Schema pattern.
func validateFormat(field, value string, pattern *regexp.Regexp) error {
	if !pattern.MatchString(value) {
		return fmt.Errorf("invalid %s %q: must match pattern %s", field, value, pattern.String())
	}
	return nil
}

// Validate checks that required fields are present and well-formed.
// Version is always required. Format is enforced on all present fields.
func (c *ComplyPackConfig) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("missing required field: version")
	}

	if err := c.validateFormats(); err != nil {
		return err
	}

	for i, schema := range c.Schemas {
		if schema.Platform == "" {
			return fmt.Errorf("schema %d missing required field: platform", i)
		}
	}

	return nil
}

// ValidateForMCP checks fields required for MCP server operation.
// id, evaluator-id, and version are optional for MCP (can be provided
// via CLI flags), but format is enforced when present.
func (c *ComplyPackConfig) ValidateForMCP() error {
	if err := c.validateFormats(); err != nil {
		return err
	}

	for i, schema := range c.Schemas {
		if schema.Platform == "" {
			return fmt.Errorf("schema %d missing required field: platform", i)
		}
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

// ValidateForInit checks fields required to produce a usable config file.
// This is the union of ValidateForPack and ValidateForMCP: the generated
// config must be immediately usable by both pack and mcp serve.
func (c *ComplyPackConfig) ValidateForInit() error {
	if c.ID == "" {
		return fmt.Errorf("missing required field: id")
	}
	if c.EvaluatorID == "" {
		return fmt.Errorf("missing required field: evaluator-id")
	}
	if c.Version == "" {
		return fmt.Errorf("missing required field: version")
	}

	if err := c.validateFormats(); err != nil {
		return err
	}

	if len(c.Schemas) == 0 {
		return fmt.Errorf("missing required field: schemas (at least one required)")
	}
	for i, schema := range c.Schemas {
		if schema.Platform == "" {
			return fmt.Errorf("schema %d missing required field: platform", i)
		}
	}
	if len(c.Gemara.Sources) == 0 {
		return fmt.Errorf("missing required field: gemara.sources (at least one source required)")
	}
	return nil
}

// BuildConfig assembles a ComplyPackConfig from parsed component values.
// This is the single assembly point for both init and mcp serve commands.
func BuildConfig(id, evaluatorID, version string, sources []GemaraSourceEntry, schemas []SchemaRef) *ComplyPackConfig {
	return &ComplyPackConfig{
		ID:          id,
		EvaluatorID: evaluatorID,
		Version:     version,
		Gemara:      GemaraConfig{Sources: sources},
		Schemas:     schemas,
	}
}

// configSchemaDirective is the yaml-language-server schema line.
const configSchemaDirective = "# yaml-language-server: $schema=https://complytime.github.io/complypack/schemas/complypack.schema.json\n"

// ConfigHeader builds the comment block prepended to generated complypack.yaml files.
// It includes the complypack version and the date the file was generated.
func ConfigHeader(complypackVersion, date string) string {
	return configSchemaDirective + fmt.Sprintf("# Generated by complypack init %s on %s\n", complypackVersion, date)
}

// MarshalConfigYAML serializes a ComplyPackConfig to YAML with a header comment
// and 2-space indentation matching the project convention.
// complypackVersion is the tool version (e.g. "1.0.0" or "(devel)") and
// date is the generation date in YYYY-MM-DD format.
func MarshalConfigYAML(cfg *ComplyPackConfig, complypackVersion, date string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(ConfigHeader(complypackVersion, date))
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteConfigFile writes data to path with safety checks for existing files
// and symlinks. When force is false, it refuses to overwrite existing files
// or follow symlinks. When createParents is true, missing parent directories
// are created automatically.
func WriteConfigFile(path string, data []byte, force, createParents bool) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "/" {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if !createParents {
				return fmt.Errorf("directory %s does not exist; use --parents to create it", dir)
			}
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("creating parent directories for %s: %w", path, err)
			}
		}
	}

	info, err := os.Lstat(path)
	exists := err == nil
	if exists {
		if info.Mode()&os.ModeSymlink != 0 && !force {
			return fmt.Errorf("%s is a symbolic link; resolve it manually or use --force", path)
		}
		if !force {
			return fmt.Errorf("%s already exists; use --force to overwrite", path)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking %s: %w", path, err)
	}

	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	f, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		if exists {
			return fmt.Errorf("opening %s for overwrite: %w", path, err)
		}
		return fmt.Errorf("creating %s: %w", path, err)
	}

	_, writeErr := f.Write(data)
	if closeErr := f.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		return fmt.Errorf("writing %s: %w", path, writeErr)
	}
	return nil
}

// CheckCredentialURIs inspects source URIs for embedded credentials
// (userinfo component per RFC 3986 §3.2.1). When allowCredentials is
// false, the first URI containing credentials causes an error. When
// true, a warning is written to w instead.
func CheckCredentialURIs(w io.Writer, sources []string, allowCredentials bool) error {
	for _, s := range sources {
		u, err := url.Parse(s)
		if err != nil {
			continue
		}
		if u.User != nil {
			if !allowCredentials {
				return fmt.Errorf("source %q contains embedded credentials; use a credential helper or pass --allow-credentials to override", s)
			}
			fmt.Fprintf(w, "WARNING: source %q contains embedded credentials; consider using a credential helper instead\n", s)
		}
	}
	return nil
}
