// SPDX-License-Identifier: Apache-2.0

package schemas

import (
	_ "embed"
	"fmt"
	"sort"

	"github.com/complytime/complypack/internal/config"
	"gopkg.in/yaml.v3"
)

//go:embed index.yaml
var indexBytes []byte

// IndexEntry represents a platform's default schema source.
type IndexEntry struct {
	Source string `yaml:"source"`
}

// LoadIndex parses the embedded schema index.
func LoadIndex() (map[string]IndexEntry, error) {
	var index map[string]IndexEntry
	if err := yaml.Unmarshal(indexBytes, &index); err != nil {
		return nil, fmt.Errorf("parsing schema index: %w", err)
	}
	return index, nil
}

// Platforms returns a sorted list of all platform names in the index.
func Platforms() []string {
	index, err := LoadIndex()
	if err != nil {
		return nil
	}
	platforms := make([]string, 0, len(index))
	for name := range index {
		platforms = append(platforms, name)
	}
	sort.Strings(platforms)
	return platforms
}

// ResolveSource returns the effective schema source for a SchemaRef,
// checking explicit source, legacy path, and index defaults in order.
func ResolveSource(ref config.SchemaRef, index map[string]IndexEntry) string {
	if ref.Source != "" {
		return ref.Source
	}
	if ref.Path != "" {
		return "file://" + ref.Path
	}
	if entry, ok := index[ref.Platform]; ok {
		return entry.Source
	}
	return ""
}
