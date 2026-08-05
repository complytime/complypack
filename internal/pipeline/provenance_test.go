// SPDX-License-Identifier: Apache-2.0

package pipeline_test

import (
	"testing"

	"github.com/complytime/complypack/internal/pipeline"
	"github.com/complytime/complypack/internal/requirement"
	"github.com/complytime/complypack/pkg/complypack"
	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolvedPolicy builds a ResolvedPolicy carrying only the Policy metadata
// and imports needed by BuildProvenance (mapping references + import refs).
func resolvedPolicy(id string, refs []gemara.MappingReference, catalogRefs, guidanceRefs []string) *requirement.ResolvedPolicy {
	catalogs := make([]gemara.CatalogImport, len(catalogRefs))
	for i, r := range catalogRefs {
		catalogs[i] = gemara.CatalogImport{ReferenceId: r}
	}
	guidance := make([]gemara.GuidanceImport, len(guidanceRefs))
	for i, r := range guidanceRefs {
		guidance[i] = gemara.GuidanceImport{ReferenceId: r}
	}
	return &requirement.ResolvedPolicy{
		Policy: gemara.Policy{
			Metadata: gemara.Metadata{
				Id:                id,
				MappingReferences: refs,
			},
			Imports: gemara.Imports{
				Catalogs: catalogs,
				Guidance: guidance,
			},
		},
	}
}

func TestBuildProvenance(t *testing.T) {
	t.Run("one entry per resolved policy, refs from imports", func(t *testing.T) {
		resolved := map[string]*requirement.ResolvedPolicy{
			"policy-a": resolvedPolicy(
				"policy-a",
				[]gemara.MappingReference{
					{Id: "cat-1", Url: "https://example.com/cat.yaml", Version: "1.0"},
					{Id: "guid-1", Url: "https://example.com/guid.yaml", Version: "2.0"},
				},
				[]string{"cat-1"},
				[]string{"guid-1"},
			),
		}

		got := pipeline.BuildProvenance(resolved, nil)

		want := []complypack.Provenance{
			{
				PolicyID: "policy-a",
				GemaraContent: []complypack.GemaraRef{
					{ReferenceID: "cat-1", URI: "https://example.com/cat.yaml", Version: "1.0"},
					{ReferenceID: "guid-1", URI: "https://example.com/guid.yaml", Version: "2.0"},
				},
			},
		}
		assert.Equal(t, want, got)
	})

	t.Run("cardinality: multiple policies never cross-product", func(t *testing.T) {
		resolved := map[string]*requirement.ResolvedPolicy{
			"policy-a": resolvedPolicy("policy-a",
				[]gemara.MappingReference{{Id: "cat-1", Url: "u1", Version: "1"}},
				[]string{"cat-1"}, nil),
			"policy-b": resolvedPolicy("policy-b",
				[]gemara.MappingReference{{Id: "cat-2", Url: "u2", Version: "2"}},
				[]string{"cat-2"}, nil),
		}

		got := pipeline.BuildProvenance(resolved, nil)

		require.Len(t, got, 2, "one entry per distinct policy, not sources*policies")
		assert.Equal(t, "policy-a", got[0].PolicyID)
		assert.Equal(t, "policy-b", got[1].PolicyID)
	})

	t.Run("determinism: non-sorted input yields sorted golden output", func(t *testing.T) {
		// Use >=3 policy keys so a MISSING policy-level sort fails
		// deterministically rather than passing ~50% of the time by luck of
		// Go's per-process map iteration order.
		resolved := map[string]*requirement.ResolvedPolicy{
			"z-policy": resolvedPolicy("z-policy",
				[]gemara.MappingReference{
					{Id: "z-ref", Url: "https://example.com/z", Version: "9"},
					{Id: "a-ref", Url: "https://example.com/a", Version: "1"},
				},
				[]string{"z-ref", "a-ref"}, nil),
			"m-policy": resolvedPolicy("m-policy",
				[]gemara.MappingReference{{Id: "m-ref", Url: "https://example.com/m", Version: "5"}},
				[]string{"m-ref"}, nil),
			"a-policy": resolvedPolicy("a-policy",
				[]gemara.MappingReference{{Id: "c-ref", Url: "https://example.com/c", Version: "3"}},
				[]string{"c-ref"}, nil),
		}

		want := []complypack.Provenance{
			{
				PolicyID: "a-policy",
				GemaraContent: []complypack.GemaraRef{
					{ReferenceID: "c-ref", URI: "https://example.com/c", Version: "3"},
				},
			},
			{
				PolicyID: "m-policy",
				GemaraContent: []complypack.GemaraRef{
					{ReferenceID: "m-ref", URI: "https://example.com/m", Version: "5"},
				},
			},
			{
				PolicyID: "z-policy",
				GemaraContent: []complypack.GemaraRef{
					{ReferenceID: "a-ref", URI: "https://example.com/a", Version: "1"},
					{ReferenceID: "z-ref", URI: "https://example.com/z", Version: "9"},
				},
			},
		}

		// Repeat to defeat any single-iteration luck if a sort is dropped.
		for i := 0; i < 20; i++ {
			assert.Equal(t, want, pipeline.BuildProvenance(resolved, nil))
		}
	})

	t.Run("url-less import records empty URI but keeps entry", func(t *testing.T) {
		resolved := map[string]*requirement.ResolvedPolicy{
			"p": resolvedPolicy("p",
				[]gemara.MappingReference{{Id: "local", Url: "", Version: "1"}},
				[]string{"local"}, nil),
		}

		got := pipeline.BuildProvenance(resolved, nil)

		require.Len(t, got, 1)
		require.Len(t, got[0].GemaraContent, 1)
		assert.Equal(t, "local", got[0].GemaraContent[0].ReferenceID)
		assert.Empty(t, got[0].GemaraContent[0].URI)
		assert.Equal(t, "1", got[0].GemaraContent[0].Version)
	})

	t.Run("URI sanitization strips credentials/query/fragment", func(t *testing.T) {
		resolved := map[string]*requirement.ResolvedPolicy{
			"p": resolvedPolicy("p",
				[]gemara.MappingReference{
					{Id: "r", Url: "https://user:secret@example.com/cat.yaml?token=abc#frag", Version: "1"}, //nolint:gosec // test fixture, not real credentials
				},
				[]string{"r"}, nil),
		}

		got := pipeline.BuildProvenance(resolved, nil)

		require.Len(t, got, 1)
		require.Len(t, got[0].GemaraContent, 1)
		assert.Equal(t, "https://example.com/cat.yaml", got[0].GemaraContent[0].URI)
	})

	t.Run("file:// URI is omitted (local path not recorded)", func(t *testing.T) {
		resolved := map[string]*requirement.ResolvedPolicy{
			"p": resolvedPolicy("p",
				[]gemara.MappingReference{
					{Id: "r", Url: "file:///home/user/secret/cat.yaml", Version: "1"},
				},
				[]string{"r"}, nil),
		}

		got := pipeline.BuildProvenance(resolved, nil)

		require.Len(t, got, 1)
		require.Len(t, got[0].GemaraContent, 1)
		assert.Empty(t, got[0].GemaraContent[0].URI, "local file path must not be recorded")
		assert.Equal(t, "r", got[0].GemaraContent[0].ReferenceID)
	})

	t.Run("import-less resolved policy yields no entry", func(t *testing.T) {
		// A policy that imports zero catalogs/guidance has no Gemara
		// provenance to record. It must NOT produce a Provenance with empty
		// GemaraContent, because Config.Validate rejects empty gemara-content
		// and Pack would then fail on machine-generated data.
		resolved := map[string]*requirement.ResolvedPolicy{
			"lonely": resolvedPolicy("lonely", nil, nil, nil),
		}

		got := pipeline.BuildProvenance(resolved, nil)

		assert.Empty(t, got, "import-less policy must be skipped, not emitted with empty gemara-content")
	})

	t.Run("import-less policy skipped, importing policy kept", func(t *testing.T) {
		resolved := map[string]*requirement.ResolvedPolicy{
			"lonely": resolvedPolicy("lonely", nil, nil, nil),
			"real": resolvedPolicy("real",
				[]gemara.MappingReference{{Id: "cat-1", Url: "https://example.com/c", Version: "1"}},
				[]string{"cat-1"}, nil),
		}

		got := pipeline.BuildProvenance(resolved, nil)

		require.Len(t, got, 1, "only the importing policy is recorded")
		assert.Equal(t, "real", got[0].PolicyID)
	})

	t.Run("import ref-id with no matching mapping reference: entry emitted with empty URI and Version", func(t *testing.T) {
		resolved := map[string]*requirement.ResolvedPolicy{
			"p": resolvedPolicy("p",
				[]gemara.MappingReference{{Id: "known", Url: "https://example.com/k", Version: "1"}},
				[]string{"orphan"}, nil),
		}

		got := pipeline.BuildProvenance(resolved, nil)

		require.Len(t, got, 1)
		require.Len(t, got[0].GemaraContent, 1)
		assert.Equal(t, "orphan", got[0].GemaraContent[0].ReferenceID)
		assert.Empty(t, got[0].GemaraContent[0].URI, "no matching mapping reference => empty URI")
		assert.Empty(t, got[0].GemaraContent[0].Version, "no matching mapping reference => empty Version")
	})

	t.Run("empty resolved map yields empty slice", func(t *testing.T) {
		got := pipeline.BuildProvenance(map[string]*requirement.ResolvedPolicy{}, nil)
		assert.Empty(t, got)
	})

	t.Run("nil resolved map yields empty slice", func(t *testing.T) {
		got := pipeline.BuildProvenance(nil, nil)
		assert.Empty(t, got)
	})

	// sanitizeURI negative branches, exercised through BuildProvenance (its
	// only caller). Each malformed/local URI must yield an empty URI while
	// the reference entry is still recorded.
	t.Run("URI negative cases record entry with empty URI", func(t *testing.T) {
		cases := []struct {
			name string
			url  string
		}{
			{"unparseable URL", "://not a url"},
			{"control character fails parse", "https://exa\x7fmple.com/cat.yaml"},
			{"scheme-relative host is omitted", "//example.com/cat.yaml"},
			{"relative path has no scheme or host", "catalogs/controls.yaml"},
			{"opaque non-file scheme has no host", "mailto:owner@example.com"},
			{"mixed-case FILE scheme is omitted", "FILE:///home/user/secret/cat.yaml"},
			{"uppercase FILE scheme is omitted", "FILE:///etc/policy.yaml"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				resolved := map[string]*requirement.ResolvedPolicy{
					"p": resolvedPolicy("p",
						[]gemara.MappingReference{{Id: "r", Url: tc.url, Version: "1"}},
						[]string{"r"}, nil),
				}
				got := pipeline.BuildProvenance(resolved, nil)
				require.Len(t, got, 1)
				require.Len(t, got[0].GemaraContent, 1)
				assert.Empty(t, got[0].GemaraContent[0].URI,
					"malformed/local URI %q must not be recorded", tc.url)
				assert.Equal(t, "r", got[0].GemaraContent[0].ReferenceID,
					"entry is still emitted when URI is omitted")
			})
		}
	})

	t.Run("credentials in URI with port are stripped but host:port kept", func(t *testing.T) {
		resolved := map[string]*requirement.ResolvedPolicy{
			"p": resolvedPolicy("p",
				[]gemara.MappingReference{
					{Id: "r", Url: "https://user:secret@registry.example.com:5000/cat.yaml", Version: "1"}, //nolint:gosec // test fixture, not real credentials
				},
				[]string{"r"}, nil),
		}
		got := pipeline.BuildProvenance(resolved, nil)
		require.Len(t, got, 1)
		require.Len(t, got[0].GemaraContent, 1)
		assert.Equal(t, "https://registry.example.com:5000/cat.yaml", got[0].GemaraContent[0].URI)
	})

	t.Run("same reference-id imported as both catalog and guidance yields two sorted entries", func(t *testing.T) {
		resolved := map[string]*requirement.ResolvedPolicy{
			"p": resolvedPolicy("p",
				[]gemara.MappingReference{
					{Id: "shared", Url: "https://example.com/s", Version: "1"},
				},
				[]string{"shared"},
				[]string{"shared"}),
		}
		got := pipeline.BuildProvenance(resolved, nil)
		require.Len(t, got, 1)
		require.Len(t, got[0].GemaraContent, 2, "a ref imported twice yields two entries")
		assert.Equal(t, "shared", got[0].GemaraContent[0].ReferenceID)
		assert.Equal(t, "shared", got[0].GemaraContent[1].ReferenceID)
		assert.Equal(t, "https://example.com/s", got[0].GemaraContent[0].URI)
		assert.Equal(t, "https://example.com/s", got[0].GemaraContent[1].URI)
	})

	t.Run("OCI bundle source records bundle reference, not mapping-reference URLs", func(t *testing.T) {
		resolved := map[string]*requirement.ResolvedPolicy{
			"policy-a": resolvedPolicy("policy-a",
				[]gemara.MappingReference{
					{Id: "cat-1", Url: "https://assembly-time.example.com/cat.yaml", Version: "1.0"},
					{Id: "guid-1", Url: "https://assembly-time.example.com/guid.yaml", Version: "2.0"},
				},
				[]string{"cat-1"},
				[]string{"guid-1"},
			),
		}
		policySources := map[string]string{
			"policy-a": "ghcr.io/org/bundle:v1.0",
		}

		got := pipeline.BuildProvenance(resolved, policySources)

		want := []complypack.Provenance{
			{
				PolicyID: "policy-a",
				GemaraContent: []complypack.GemaraRef{
					{ReferenceID: "cat-1", URI: "ghcr.io/org/bundle:v1.0", Version: "1.0"},
					{ReferenceID: "guid-1", URI: "ghcr.io/org/bundle:v1.0", Version: "2.0"},
				},
			},
		}
		assert.Equal(t, want, got)
	})

	t.Run("OCI bundle source with oci:// scheme records bare reference", func(t *testing.T) {
		resolved := map[string]*requirement.ResolvedPolicy{
			"p": resolvedPolicy("p",
				[]gemara.MappingReference{
					{Id: "cat-1", Url: "https://assembly-time.example.com/cat.yaml", Version: "1.0"},
				},
				[]string{"cat-1"}, nil,
			),
		}
		policySources := map[string]string{
			"p": "oci://ghcr.io/org/bundle:v2.0",
		}

		got := pipeline.BuildProvenance(resolved, policySources)

		require.Len(t, got, 1)
		require.Len(t, got[0].GemaraContent, 1)
		assert.Equal(t, "ghcr.io/org/bundle:v2.0", got[0].GemaraContent[0].URI,
			"oci:// prefix must be stripped from the recorded reference")
	})

	t.Run("OCI bundle source with credentials records redacted reference", func(t *testing.T) {
		resolved := map[string]*requirement.ResolvedPolicy{
			"p": resolvedPolicy("p",
				[]gemara.MappingReference{
					{Id: "cat-1", Url: "https://assembly-time.example.com/cat.yaml", Version: "1.0"},
				},
				[]string{"cat-1"}, nil,
			),
		}
		policySources := map[string]string{ //nolint:gosec // test fixture, not real credentials
			"p": "oci://user:secret@ghcr.io/org/bundle:v1.0",
		}

		got := pipeline.BuildProvenance(resolved, policySources)

		require.Len(t, got, 1)
		require.Len(t, got[0].GemaraContent, 1)
		assert.Equal(t, "ghcr.io/org/bundle:v1.0", got[0].GemaraContent[0].URI,
			"credentials must be stripped from OCI bundle reference")
	})

	t.Run("file source uses MappingReference.Url, not source path", func(t *testing.T) {
		resolved := map[string]*requirement.ResolvedPolicy{
			"p": resolvedPolicy("p",
				[]gemara.MappingReference{
					{Id: "cat-1", Url: "https://example.com/cat.yaml", Version: "1.0"},
				},
				[]string{"cat-1"}, nil,
			),
		}
		policySources := map[string]string{
			"p": "file:///home/user/catalogs/policy.yaml",
		}

		got := pipeline.BuildProvenance(resolved, policySources)

		require.Len(t, got, 1)
		require.Len(t, got[0].GemaraContent, 1)
		assert.Equal(t, "https://example.com/cat.yaml", got[0].GemaraContent[0].URI,
			"file sources must use MappingReference.Url, not the file path")
	})

	t.Run("mixed OCI and file sources record correct URIs per policy", func(t *testing.T) {
		resolved := map[string]*requirement.ResolvedPolicy{
			"oci-policy": resolvedPolicy("oci-policy",
				[]gemara.MappingReference{
					{Id: "cat-1", Url: "https://assembly-time.example.com/cat.yaml", Version: "1.0"},
				},
				[]string{"cat-1"}, nil,
			),
			"file-policy": resolvedPolicy("file-policy",
				[]gemara.MappingReference{
					{Id: "cat-2", Url: "https://example.com/cat2.yaml", Version: "2.0"},
				},
				[]string{"cat-2"}, nil,
			),
		}
		policySources := map[string]string{
			"oci-policy":  "ghcr.io/org/bundle:v1.0",
			"file-policy": "/local/path/policy.yaml",
		}

		got := pipeline.BuildProvenance(resolved, policySources)

		require.Len(t, got, 2)
		assert.Equal(t, "file-policy", got[0].PolicyID)
		assert.Equal(t, "https://example.com/cat2.yaml", got[0].GemaraContent[0].URI,
			"file-sourced policy must use MappingReference.Url")
		assert.Equal(t, "oci-policy", got[1].PolicyID)
		assert.Equal(t, "ghcr.io/org/bundle:v1.0", got[1].GemaraContent[0].URI,
			"OCI-sourced policy must use the bundle reference")
	})
}
