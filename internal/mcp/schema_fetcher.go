// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// fetchSchemaFromURL downloads a schema from an HTTP(S) URL.
// Returns the schema bytes and detected format.
func fetchSchemaFromURL(ctx context.Context, url string) ([]byte, SchemaFormat, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, FormatUnknown, fmt.Errorf("creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, FormatUnknown, fmt.Errorf("fetching URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, FormatUnknown, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, FormatUnknown, fmt.Errorf("reading response: %w", err)
	}

	format := DetectFormat(url)
	return data, format, nil
}

// loadSchemaFromFile reads a schema from a local file path.
// Returns the schema bytes and detected format.
func loadSchemaFromFile(path string) ([]byte, SchemaFormat, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, FormatUnknown, fmt.Errorf("reading file: %w", err)
	}

	format := DetectFormat(path)
	return data, format, nil
}

// buildCUEFromBytes compiles CUE bytes into a cue.Value.
func buildCUEFromBytes(data []byte) (cue.Value, error) {
	ctx := cuecontext.New()
	value := ctx.CompileBytes(data)
	if err := value.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("compiling CUE: %w", err)
	}
	return value, nil
}
