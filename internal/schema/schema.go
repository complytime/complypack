// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"context"
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// Schema holds a loaded platform schema with both raw bytes (for serving)
// and compiled CUE (for validation).
type Schema struct {
	Platform string
	Bytes    []byte
	CUE      cue.Value
}

// Loader loads a schema from a source string.
type Loader interface {
	Match(source string) bool
	Load(ctx context.Context, source string, platform string) (*Schema, error)
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

// IsJSONSchema returns true if the data looks like JSON (starts with '{').
func IsJSONSchema(data []byte) bool {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '{':
			return true
		default:
			return false
		}
	}
	return false
}

// FormatCUEDefinitions extracts top-level definitions from a CUE value and
// formats them as readable CUE syntax.
func FormatCUEDefinitions(val cue.Value) []byte {
	var sb strings.Builder
	iter, _ := val.Fields(cue.Definitions(true))
	for iter.Next() {
		label := iter.Selector().String()
		defVal := iter.Value()
		fmt.Fprintf(&sb, "%s: %v\n\n", label, defVal)
	}
	return []byte(sb.String())
}

// ValidateData validates arbitrary data against a compiled CUE schema
// using CUE unification. Returns nil when data conforms to the schema,
// or a list of human-readable error strings describing violations.
func ValidateData(
	data map[string]interface{}, cueSchema cue.Value,
) []string {
	cueCtx := cuecontext.New()
	dataVal := cueCtx.Encode(data)
	if dataVal.Err() != nil {
		return []string{
			fmt.Sprintf(
				"failed to encode data: %v", dataVal.Err(),
			),
		}
	}

	unified := cueSchema.Unify(dataVal)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return collectCUEErrors(err)
	}

	return nil
}

// collectCUEErrors extracts individual error messages from a CUE
// error, which may wrap multiple validation failures.
func collectCUEErrors(err error) []string {
	type errorList interface {
		Unwrap() []error
	}

	var errors []string
	if el, ok := err.(errorList); ok {
		for _, e := range el.Unwrap() {
			errors = append(errors, e.Error())
		}
	} else {
		errors = append(errors, err.Error())
	}
	return errors
}

// BuildCUEFromBytes compiles CUE bytes into a cue.Value.
func BuildCUEFromBytes(data []byte) (cue.Value, error) {
	ctx := cuecontext.New()
	value := ctx.CompileBytes(data)
	if err := value.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("compiling CUE: %w", err)
	}
	return value, nil
}
