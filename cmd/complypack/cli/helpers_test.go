// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/complytime/complypack/internal/pipeline"
	"github.com/complytime/complypack/internal/requirement"
	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Shared policy fixtures for CLI tests ---

// regoValidPolicyCLI is a syntactically valid Rego
// policy for CLI tests.
const regoValidPolicyCLI = "package cli.valid\n\nimport rego.v1\n\ndeny contains msg if {\n\tinput.kind == \"Pod\"\n\tnot input.metadata.name\n\tmsg := \"Pods must have a name\"\n}"

// regoSyntaxErrorPolicyCLI has a deliberate syntax
// error (missing closing brace).
const regoSyntaxErrorPolicyCLI = "package cli.syntax_error\n\nimport rego.v1\n\ndeny contains msg if {\n\tinput.kind == \"Pod\"\n\tnot input.metadata.labels[\"app\"]\n\tmsg := \"Pods must have 'app' label\"\n  # Missing closing brace"

// regoContractViolationPolicyCLI references a field
// not in the schema.
const regoContractViolationPolicyCLI = "package cli.contract_violation\n\nimport rego.v1\n\ndeny contains msg if {\n\tinput.kind == \"Pod\"\n\t# This field doesn't exist in Kubernetes schema\n\tnot input.metadata.invalid_field\n\tmsg := \"Contract violation example\"\n}"

func writePolicyFile(
	t *testing.T, dir, content string,
) string {
	t.Helper()
	path := filepath.Join(dir, "policy.rego")
	require.NoError(t,
		os.WriteFile(path, []byte(content), 0600))
	return path
}

func TestFindResolvedPolicyCLI_ResolvedPolicy(t *testing.T) {
	rp := &requirement.ResolvedPolicy{
		Policy: gemara.Policy{
			Metadata: gemara.Metadata{Id: "my-policy"},
		},
	}
	lr := &pipeline.LoadResult{
		Artifacts: requirement.NewArtifactSet(),
		Resolved:  map[string]*requirement.ResolvedPolicy{"my-policy": rp},
	}

	result, err := findResolvedPolicyCLI(lr, "my-policy")
	require.NoError(t, err)
	assert.Equal(t, "my-policy", result.Policy.Metadata.Id)
}

func TestFindResolvedPolicyCLI_CatalogFallback(t *testing.T) {
	cat := &gemara.ControlCatalog{
		Metadata: gemara.Metadata{Id: "my-catalog"},
		Controls: []gemara.Control{
			{Id: "CTL-001"},
		},
	}
	artifacts := requirement.NewArtifactSet()
	artifacts.Catalogs["my-catalog"] = cat

	lr := &pipeline.LoadResult{
		Artifacts: artifacts,
		Resolved:  map[string]*requirement.ResolvedPolicy{},
	}

	result, err := findResolvedPolicyCLI(lr, "my-catalog")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestFindResolvedPolicyCLI_NotFound(t *testing.T) {
	lr := &pipeline.LoadResult{
		Artifacts: requirement.NewArtifactSet(),
		Resolved:  map[string]*requirement.ResolvedPolicy{},
	}

	_, err := findResolvedPolicyCLI(lr, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(),
		`"nonexistent" not found in loaded artifacts`)
}

func TestFindResolvedPolicyCLI_PrefersResolved(t *testing.T) {
	// When both a resolved policy and a catalog exist with
	// the same name, the resolved policy takes precedence.
	rp := &requirement.ResolvedPolicy{
		Policy: gemara.Policy{
			Metadata: gemara.Metadata{Id: "shared-name"},
		},
	}
	cat := &gemara.ControlCatalog{
		Metadata: gemara.Metadata{Id: "shared-name"},
	}
	artifacts := requirement.NewArtifactSet()
	artifacts.Catalogs["shared-name"] = cat

	lr := &pipeline.LoadResult{
		Artifacts: artifacts,
		Resolved: map[string]*requirement.ResolvedPolicy{
			"shared-name": rp,
		},
	}

	result, err := findResolvedPolicyCLI(lr, "shared-name")
	require.NoError(t, err)
	assert.Equal(t, "shared-name", result.Policy.Metadata.Id,
		"should return the resolved policy, not re-resolve the catalog")
}

func TestLoadArtifacts_MissingConfig(t *testing.T) {
	_, err := loadArtifacts(
		context.Background(),
		nil,
		"/nonexistent/complypack.yaml",
		"",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load config")
}

func TestLoadArtifacts_EmptySource(t *testing.T) {
	_, err := loadArtifacts(
		context.Background(),
		[]string{""},
		"",
		"",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty source flag value")
}
