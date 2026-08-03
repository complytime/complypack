// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"net/url"
	"sort"
	"strings"

	"github.com/complytime/complypack/internal/requirement"
	"github.com/complytime/complypack/pkg/complypack"
	"github.com/gemaraproj/go-gemara"
)

// BuildProvenance maps resolved policies to the provenance records recorded
// in a pack's OCI config blob. It produces at most one entry per resolved
// policy (never a cross-product of sources and policies), listing the Gemara
// artifacts that policy imports. Policies that import no catalogs or guidance
// have no provenance to record and are skipped entirely.
//
// For each imported catalog and guidance reference, the corresponding
// MappingReference (matched by MappingReference.Id) supplies the URI and
// version. Output is deterministically ordered: entries by PolicyID and
// references by ReferenceID, so identical resolved content always yields a
// byte-identical config blob.
//
// URIs are sanitized before recording into the published, publicly
// distributable blob (CWE-200): embedded credentials, query strings, and
// fragments are stripped, and local (file://) paths are omitted entirely.
// The reference entry is always emitted even when its URI is omitted.
func BuildProvenance(resolved map[string]*requirement.ResolvedPolicy) []complypack.Provenance {
	provenance := make([]complypack.Provenance, 0, len(resolved))

	for _, rp := range resolved {
		refIndex := indexMappingReferences(rp.Policy.Metadata.MappingReferences)

		var refs []complypack.GemaraRef
		for _, imp := range rp.Policy.Imports.Catalogs {
			refs = append(refs, buildRef(imp.ReferenceId, refIndex))
		}
		for _, imp := range rp.Policy.Imports.Guidance {
			refs = append(refs, buildRef(imp.ReferenceId, refIndex))
		}

		// An import-less policy has no Gemara provenance to record. Skip it
		// rather than emit a Provenance with empty GemaraContent, which
		// Config.Validate rejects (Pack would then fail on generated data).
		if len(refs) == 0 {
			continue
		}

		sort.Slice(refs, func(i, j int) bool {
			return refs[i].ReferenceID < refs[j].ReferenceID
		})

		provenance = append(provenance, complypack.Provenance{
			PolicyID:      rp.Policy.Metadata.Id,
			GemaraContent: refs,
		})
	}

	sort.Slice(provenance, func(i, j int) bool {
		return provenance[i].PolicyID < provenance[j].PolicyID
	})

	return provenance
}

// indexMappingReferences maps a policy's mapping-reference IDs to their
// full mapping reference for URI/version lookup.
func indexMappingReferences(refs []gemara.MappingReference) map[string]gemara.MappingReference {
	idx := make(map[string]gemara.MappingReference, len(refs))
	for _, ref := range refs {
		idx[ref.Id] = ref
	}
	return idx
}

// buildRef resolves an import reference-id to a recorded GemaraRef. The URI
// is sanitized; the entry is always emitted even if no matching mapping
// reference is found or its URI is omitted.
func buildRef(referenceID string, refIndex map[string]gemara.MappingReference) complypack.GemaraRef {
	ref := complypack.GemaraRef{ReferenceID: referenceID}
	if mr, ok := refIndex[referenceID]; ok {
		ref.URI = sanitizeURI(mr.Url)
		ref.Version = mr.Version
	}
	return ref
}

// sanitizeURI strips credentials, query strings, and fragments from a URI
// before it is recorded into the published config blob (CWE-200). Local
// file:// URIs and unparseable/relative values are omitted entirely so that
// local filesystem layout is never leaked.
func sanitizeURI(raw string) string {
	if raw == "" {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	// Local paths must never be recorded into a published artifact.
	if u.Scheme == "file" || u.Scheme == "" || u.Host == "" {
		return ""
	}

	sanitized := url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   u.Path,
	}
	return strings.TrimSuffix(sanitized.String(), "/")
}
