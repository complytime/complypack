// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"testing"

	"github.com/complytime/complypack/internal/pipeline"
	"github.com/complytime/complypack/internal/requirement"
	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
