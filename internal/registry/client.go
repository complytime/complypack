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
		return nil, fmt.Errorf("invalid OCI reference %q: %w", ref, err)
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
