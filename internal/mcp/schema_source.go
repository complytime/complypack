// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"fmt"
	"strings"
)

// SchemaSourceType identifies the type of schema source.
type SchemaSourceType int

const (
	SourceTypeUnknown SchemaSourceType = iota
	SourceTypeCUEModule
	SourceTypeHTTPS
	SourceTypeHTTP
	SourceTypeFile
	SourceTypeLegacyPath // Backward compat for path field
)

// SchemaSource represents a parsed schema source URI.
type SchemaSource struct {
	Type SchemaSourceType
	Path string // Module path, URL, or file path depending on Type
}

// ParseSchemaSource parses a source URI and determines its type.
// Supported schemes:
//   - cue://module.path
//   - https://example.com/schema.json
//   - http://example.com/schema.json
//   - file://./path/to/file
//
// Empty source returns SourceTypeUnknown (will use embedded fallback).
func ParseSchemaSource(source string) (SchemaSource, error) {
	if source == "" {
		return SchemaSource{Type: SourceTypeUnknown}, nil
	}

	// Check for URI schemes
	if strings.HasPrefix(source, "cue://") {
		modulePath := strings.TrimPrefix(source, "cue://")
		if modulePath == "" {
			return SchemaSource{}, fmt.Errorf("cue:// scheme requires module path")
		}
		return SchemaSource{
			Type: SourceTypeCUEModule,
			Path: modulePath,
		}, nil
	}

	if strings.HasPrefix(source, "https://") {
		return SchemaSource{
			Type: SourceTypeHTTPS,
			Path: source,
		}, nil
	}

	if strings.HasPrefix(source, "http://") {
		return SchemaSource{
			Type: SourceTypeHTTP,
			Path: source,
		}, nil
	}

	if strings.HasPrefix(source, "file://") {
		filePath := strings.TrimPrefix(source, "file://")
		if filePath == "" {
			return SchemaSource{}, fmt.Errorf("file:// scheme requires path")
		}
		return SchemaSource{
			Type: SourceTypeFile,
			Path: filePath,
		}, nil
	}

	// No recognized scheme - treat as legacy path
	return SchemaSource{
		Type: SourceTypeLegacyPath,
		Path: source,
	}, nil
}

// SchemaFormat identifies the schema file format.
type SchemaFormat int

const (
	FormatUnknown SchemaFormat = iota
	FormatJSON
	FormatCUE
)

// DetectFormat determines the schema format from file extension.
func DetectFormat(path string) SchemaFormat {
	if strings.HasSuffix(path, ".json") {
		return FormatJSON
	}
	if strings.HasSuffix(path, ".cue") {
		return FormatCUE
	}
	return FormatUnknown
}
