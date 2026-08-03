// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// NewRepository creates an authenticated remote.Repository from a full OCI reference.
// The plainHTTP parameter forces HTTP instead of HTTPS, or is auto-detected from http:// scheme.
func NewRepository(ref string, credFunc auth.CredentialFunc, plainHTTP bool) (*remote.Repository, error) {
	// Auto-detect plainHTTP from http:// scheme
	if strings.HasPrefix(ref, "http://") {
		plainHTTP = true
	}

	// Parse the reference to extract repository name
	parsedRef, err := registry.ParseReference(stripScheme(ref))
	if err != nil {
		// Redact any embedded credentials before surfacing the reference in
		// an error (CWE-209: Information Exposure Through an Error Message).
		return nil, fmt.Errorf("invalid OCI reference %q: %w", RedactCredentials(ref), err)
	}

	repoName := fmt.Sprintf("%s/%s", parsedRef.Registry, parsedRef.Repository)
	repo, err := remote.NewRepository(repoName)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository client for %s: %w", repoName, err)
	}

	repo.PlainHTTP = plainHTTP

	// Always use custom HTTP client with timeout to prevent hanging on unresponsive registries
	httpClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	if credFunc != nil {
		repo.Client = &auth.Client{
			Client:     httpClient,
			Credential: credFunc,
		}
	} else {
		// Use timeout even without credentials
		repo.Client = &http.Client{
			Timeout: 60 * time.Second,
		}
	}

	return repo, nil
}

// ParseTag extracts the tag or digest from an OCI reference.
// Returns "latest" if no tag or digest is present.
func ParseTag(ref string) string {
	ref = stripScheme(ref)

	// Check for digest reference (@sha256:...)
	if idx := strings.LastIndex(ref, "@"); idx >= 0 {
		return ref[idx+1:]
	}

	// Check for tag reference (:v1.0)
	if idx := strings.LastIndex(ref, ":"); idx >= 0 {
		candidate := ref[idx+1:]
		// Make sure the colon isn't part of the host (e.g., localhost:5000)
		if !strings.Contains(candidate, "/") {
			return candidate
		}
	}

	return "latest"
}

// stripScheme removes http:// or https:// prefix from a reference.
func stripScheme(ref string) string {
	ref = strings.TrimPrefix(ref, "http://")
	ref = strings.TrimPrefix(ref, "https://")
	return ref
}

// RedactCredentials removes any userinfo (user:password@) embedded in an OCI
// or URL reference so it can be safely surfaced in error messages or recorded
// into published artifacts (CWE-209 / CWE-200). The scheme (if any) and the
// host/path portion are preserved so operators can still identify the source.
//
// Any "scheme://" prefix is recognized generically (RFC 3986 scheme syntax),
// not just a fixed allow-list, so credentials embedded after an unlisted
// scheme (e.g. ssh://user:pass@host/path) are still stripped rather than
// leaked. Userinfo is identified as the last '@' occurring before the first
// '/' of the authority, so passwords containing '@' are fully stripped and a
// digest reference (repo@sha256:...) that places '@' after the path is left
// intact. The original scheme casing is preserved.
func RedactCredentials(ref string) string {
	scheme, rest := splitScheme(ref)

	// Userinfo, if present, is delimited by the last '@' before the first '/'.
	slash := strings.IndexByte(rest, '/')
	authority := rest
	if slash >= 0 {
		authority = rest[:slash]
	}
	if at := strings.LastIndexByte(authority, '@'); at >= 0 {
		rest = rest[at+1:]
	}

	return scheme + rest
}

// splitScheme separates a leading "scheme://" prefix from the remainder of a
// reference, returning the prefix (including "://", original casing preserved)
// and the rest. When no valid scheme prefix is present, scheme is "" and rest
// is the input unchanged. A scheme is a leading ALPHA followed by any of
// ALPHA / DIGIT / "+" / "-" / "." per RFC 3986, terminated by "://".
func splitScheme(ref string) (scheme, rest string) {
	sep := strings.Index(ref, "://")
	if sep <= 0 {
		return "", ref
	}
	for i := 0; i < sep; i++ {
		c := ref[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
			// always allowed
		case c >= '0' && c <= '9', c == '+', c == '-', c == '.':
			if i == 0 {
				return "", ref // scheme must start with a letter
			}
		default:
			return "", ref // not a valid scheme character
		}
	}
	return ref[:sep+len("://")], ref[sep+len("://"):]
}
