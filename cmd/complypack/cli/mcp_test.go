// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMcpCommand(t *testing.T) {
	root := New()

	// Find the mcp command
	mcpCmd, _, err := root.Find([]string{"mcp"})
	require.NoError(t, err, "mcp command should exist")
	assert.Equal(t, "mcp", mcpCmd.Name())
	assert.NotEmpty(t, mcpCmd.Short, "mcp command should have a short description")

	// Find the serve subcommand
	serveCmd, _, err := mcpCmd.Find([]string{"serve"})
	require.NoError(t, err, "mcp serve command should exist")
	assert.Equal(t, "serve", serveCmd.Name())
	assert.NotEmpty(t, serveCmd.Short, "serve command should have a short description")

	// Check flags exist
	flags := serveCmd.Flags()
	assert.NotNil(t, flags.Lookup("config"), "should have --config flag")
	assert.NotNil(t, flags.Lookup("cache-dir"), "should have --cache-dir flag")
	assert.NotNil(t, flags.Lookup("source"), "should have --source flag")
	assert.NotNil(t, flags.Lookup("schema"), "should have --schema flag")
}

func TestWriteStartupError(t *testing.T) {
	// writeStartupError writes to os.Stdout, so we capture via pipe.
	// This test must not run in parallel with tests that read os.Stdout.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	writeStartupError(errors.New("failed to read file: open ./missing.yaml: no such file or directory"))

	require.NoError(t, w.Close())
	os.Stdout = origStdout

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()
	output := string(buf[:n])

	// Parse the JSON-RPC response
	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      any    `json:"id"`
		Error   struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(output), &resp), "output should be valid JSON: %s", output)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.ID, "id should be null for pre-handshake errors")
	assert.Equal(t, -32603, resp.Error.Code, "should use JSON-RPC internal error code")
	assert.Contains(t, resp.Error.Message, "complypack startup failed")
	assert.Contains(t, resp.Error.Message, "missing.yaml")
}
