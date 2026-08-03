// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/internal/pipeline"
	"github.com/complytime/complypack/internal/requirement"
	"github.com/complytime/complypack/pkg/complypack"
	"github.com/gemaraproj/go-gemara"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
)

// fetchPackConfigBlob resolves the manifest referenced by desc and returns the
// raw config blob bytes as a string, so tests can assert on the exact
// serialized form published to a registry.
func fetchPackConfigBlob(t *testing.T, ctx context.Context, store content.Fetcher, desc ocispec.Descriptor) string {
	t.Helper()

	manifestBytes, err := content.FetchAll(ctx, store, desc)
	require.NoError(t, err)

	var manifest ocispec.Manifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))

	configBytes, err := content.FetchAll(ctx, store, manifest.Config)
	require.NoError(t, err)

	return string(configBytes)
}

func TestResolveProvenance(t *testing.T) {
	ctx := context.Background()

	t.Run("no gemara sources yields nil provenance and no error", func(t *testing.T) {
		cfg := &config.ComplyPackConfig{
			ID:          "io.complytime.test",
			EvaluatorID: "opa",
			Version:     "1.0.0",
		}
		prov, err := resolveProvenance(ctx, cfg, t.TempDir())
		require.NoError(t, err)
		assert.Nil(t, prov)
	})

	t.Run("unresolvable source hard-fails naming the sanitized source", func(t *testing.T) {
		cfg := &config.ComplyPackConfig{
			ID:          "io.complytime.test",
			EvaluatorID: "opa",
			Version:     "1.0.0",
			Gemara: config.GemaraConfig{
				Sources: []config.GemaraSourceEntry{
					{Source: "file:///nonexistent/catalog.yaml"},
				},
			},
		}
		prov, err := resolveProvenance(ctx, cfg, t.TempDir())
		require.Error(t, err)
		assert.Nil(t, prov)
		assert.Contains(t, err.Error(), "source file:///nonexistent/catalog.yaml")
	})

	t.Run("resolvable sources populate provenance", func(t *testing.T) {
		cfg := &config.ComplyPackConfig{
			ID:          "io.complytime.test",
			EvaluatorID: "opa",
			Version:     "1.0.0",
			Gemara: config.GemaraConfig{
				Sources: []config.GemaraSourceEntry{
					{Source: "file://../../../examples/gemara/policy.yaml"},
					{Source: "file://../../../examples/gemara/control-catalog.yaml"},
					{Source: "file://../../../examples/gemara/guidance-catalog.yaml"},
				},
			},
		}
		prov, err := resolveProvenance(ctx, cfg, t.TempDir())
		require.NoError(t, err)
		require.Len(t, prov, 1)
		assert.Equal(t, "container-platform-policy", prov[0].PolicyID)
		// The example policy imports one catalog and one guidance reference,
		// each carrying a version but no url (local mapping references record
		// no URI). Assert the exact enriched content, not merely non-empty.
		require.Len(t, prov[0].GemaraContent, 2)
		assert.Equal(t, complypack.GemaraRef{
			ReferenceID: "container-security-controls",
			Version:     "1.0.0",
		}, prov[0].GemaraContent[0])
		assert.Equal(t, complypack.GemaraRef{
			ReferenceID: "container-security-guidance",
			Version:     "1.0.0",
		}, prov[0].GemaraContent[1])
	})

	t.Run("credentialed mapping-reference URL is sanitized end to end into the config blob", func(t *testing.T) {
		// End-to-end CWE-200 guarantee: a policy whose mapping reference
		// carries a URL with embedded credentials and a query string must,
		// after BuildProvenance -> Pack -> pull, appear in the published
		// config blob with userinfo and query stripped.
		resolved := map[string]*requirement.ResolvedPolicy{
			"leaky-policy": {
				Policy: gemara.Policy{
					Metadata: gemara.Metadata{
						Id: "leaky-policy",
						MappingReferences: []gemara.MappingReference{
							{ //nolint:gosec // test fixture, not real credentials
								Id:      "cat-secret",
								Url:     "https://user:supersecret@registry.example.com/cat.yaml?token=abc#frag",
								Version: "1.0.0",
							},
						},
					},
					Imports: gemara.Imports{
						Catalogs: []gemara.CatalogImport{{ReferenceId: "cat-secret"}},
					},
				},
			},
		}

		provenance := pipeline.BuildProvenance(resolved)
		require.Len(t, provenance, 1)

		packCfg := complypack.Config{
			ID:          "io.complytime.test",
			EvaluatorID: "opa",
			Version:     "1.0.0",
			Source:      provenance,
		}

		store := memory.New()
		desc, err := complypack.Pack(ctx, store, packCfg, strings.NewReader("policy content"))
		require.NoError(t, err)

		blob := fetchPackConfigBlob(t, ctx, store, desc)
		assert.NotContains(t, blob, "supersecret",
			"published config blob must not contain the embedded credential")
		assert.NotContains(t, blob, "token=abc",
			"published config blob must not contain the query string")
		assert.Contains(t, blob, "https://registry.example.com/cat.yaml",
			"published config blob must contain the sanitized URI")
	})

	t.Run("source loads but resolves to no policy yields empty provenance and no error", func(t *testing.T) {
		// A lone control catalog loads and merges cleanly but carries no
		// policy, so Resolved is empty: pack must succeed with no provenance.
		cfg := &config.ComplyPackConfig{
			ID:          "io.complytime.test",
			EvaluatorID: "opa",
			Version:     "1.0.0",
			Gemara: config.GemaraConfig{
				Sources: []config.GemaraSourceEntry{
					{Source: "file://../../../examples/gemara/control-catalog.yaml"},
				},
			},
		}
		prov, err := resolveProvenance(ctx, cfg, t.TempDir())
		require.NoError(t, err)
		assert.Empty(t, prov)
	})
}
