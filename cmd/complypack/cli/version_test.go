// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	root := New()

	cmd, _, err := root.Find([]string{"version"})
	if err != nil {
		t.Fatalf("Find(version): %v", err)
	}
	if cmd.Name() != "version" {
		t.Errorf("Name = %q, want %q", cmd.Name(), "version")
	}
	if cmd.Short == "" {
		t.Error("version command should have a short description")
	}
	if cmd.Flags().Lookup("json") == nil {
		t.Error("--json flag not registered")
	}
}

func TestVersionCommand_PlainOutput(t *testing.T) {
	root := New()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "complypack") {
		t.Errorf("output = %q, want substring %q", out, "complypack")
	}
	if !strings.Contains(out, "dev") {
		t.Errorf("output = %q, want substring %q", out, "dev")
	}
}

func TestVersionCommand_JSONOutput(t *testing.T) {
	root := New()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"version", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var info map[string]string
	if err := json.Unmarshal(buf.Bytes(), &info); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if _, ok := info["version"]; !ok {
		t.Error("JSON output missing 'version' key")
	}
	if _, ok := info["commit"]; !ok {
		t.Error("JSON output missing 'commit' key")
	}
	if _, ok := info["gitTreeState"]; !ok {
		t.Error("JSON output missing 'gitTreeState' key")
	}
	if _, ok := info["buildDate"]; !ok {
		t.Error("JSON output missing 'buildDate' key")
	}
}
