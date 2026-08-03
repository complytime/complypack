// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"strings"
	"testing"
)

// TestNewRepositoryErrorRedactsCredentials proves the ParseReference error
// path does not leak embedded credentials into the returned error (CWE-209).
func TestNewRepositoryErrorRedactsCredentials(t *testing.T) {
	_, err := NewRepository("oci://user:supersecret@/bad ref with spaces", nil, false) //nolint:gosec // test fixture, not real credentials
	if err == nil {
		t.Fatal("expected an error for malformed reference")
	}
	if strings.Contains(err.Error(), "supersecret") {
		t.Fatalf("error leaked credentials: %v", err)
	}
}

func TestParseTag(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{
			name: "tag reference",
			ref:  "ghcr.io/org/repo:v1.0",
			want: "v1.0",
		},
		{
			name: "digest reference",
			ref:  "ghcr.io/org/repo@sha256:abc123",
			want: "sha256:abc123",
		},
		{
			name: "no tag or digest",
			ref:  "ghcr.io/org/repo",
			want: "latest",
		},
		{
			name: "with http scheme",
			ref:  "http://localhost:5000/repo:v1",
			want: "v1",
		},
		{
			name: "with https scheme",
			ref:  "https://ghcr.io/org/repo:tag",
			want: "tag",
		},
		{
			name: "port in host",
			ref:  "localhost:5000/repo",
			want: "latest",
		},
		{
			name: "port and tag",
			ref:  "localhost:5000/repo:v2.0",
			want: "v2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTag(tt.ref)
			if got != tt.want {
				t.Errorf("ParseTag(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}

func TestRedactCredentials(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{
			name: "plain file path unchanged",
			ref:  "file://catalogs/controls.yaml",
			want: "file://catalogs/controls.yaml",
		},
		{
			name: "credential-free oci reference unchanged",
			ref:  "ghcr.io/org/catalog:v1",
			want: "ghcr.io/org/catalog:v1",
		},
		{ //nolint:gosec // test fixture, not real credentials
			name: "oci scheme credentials redacted, scheme preserved",
			ref:  "oci://user:secret@ghcr.io/org/catalog:v1",
			want: "oci://ghcr.io/org/catalog:v1",
		},
		{ //nolint:gosec // test fixture, not real credentials
			name: "https credentials redacted",
			ref:  "https://user:secret@registry.example.com/org/repo:tag",
			want: "https://registry.example.com/org/repo:tag",
		},
		{ //nolint:gosec // test fixture, not real credentials
			name: "bare (scheme-less) credentials redacted",
			ref:  "user:secret@ghcr.io/org/catalog:v1",
			want: "ghcr.io/org/catalog:v1",
		},
		{ //nolint:gosec // test fixture, not real credentials
			name: "password containing @ fully redacted (last @ before slash)",
			ref:  "https://user:p@ss@registry.example.com/org/repo:tag",
			want: "https://registry.example.com/org/repo:tag",
		},
		{ //nolint:gosec // test fixture, not real credentials
			name: "uppercase scheme redacted case-insensitively, original case preserved",
			ref:  "OCI://user:secret@ghcr.io/org/catalog:v1",
			want: "OCI://ghcr.io/org/catalog:v1",
		},
		{ //nolint:gosec // test fixture, not real credentials
			name: "mixed-case scheme redacted",
			ref:  "HtTpS://user:secret@registry.example.com/org/repo:tag",
			want: "HtTpS://registry.example.com/org/repo:tag",
		},
		{ //nolint:gosec // test fixture, not real credentials
			name: "credentials with port in host redacted",
			ref:  "oci://user:secret@localhost:5000/org/repo:tag",
			want: "oci://localhost:5000/org/repo:tag",
		},
		{
			name: "digest @ after slash is not treated as userinfo",
			ref:  "ghcr.io/org/repo@sha256:abc123",
			want: "ghcr.io/org/repo@sha256:abc123",
		},
		{
			name: "empty string unchanged",
			ref:  "",
			want: "",
		},
		{ //nolint:gosec // test fixture, not real credentials
			name: "file scheme with embedded credentials is redacted",
			ref:  "file://user:secret@host/catalogs/controls.yaml",
			want: "file://host/catalogs/controls.yaml",
		},
		{ //nolint:gosec // test fixture, not real credentials
			name: "userinfo with no password is redacted",
			ref:  "oci://user@ghcr.io/org/catalog:v1",
			want: "oci://ghcr.io/org/catalog:v1",
		},
		{
			name: "empty userinfo (bare @) is stripped",
			ref:  "oci://@ghcr.io/org/catalog:v1",
			want: "oci://ghcr.io/org/catalog:v1",
		},
		{ //nolint:gosec // test fixture, not real credentials
			name: "scheme-less reference with no slash and credentials redacted",
			ref:  "user:secret@ghcr.io",
			want: "ghcr.io",
		},
		{ //nolint:gosec // test fixture, not real credentials
			name: "unlisted scheme (ssh) credentials redacted",
			ref:  "ssh://user:supersecret@host/path",
			want: "ssh://host/path",
		},
		{ //nolint:gosec // test fixture, not real credentials
			name: "unlisted scheme (git) credentials redacted",
			ref:  "git://user:secret@example.com/org/repo.git",
			want: "git://example.com/org/repo.git",
		},
		{ //nolint:gosec // test fixture, not real credentials
			name: "unlisted scheme with port and @-in-password redacted",
			ref:  "ftp://user:p@ss@host:2121/dir/file",
			want: "ftp://host:2121/dir/file",
		},
		{ //nolint:gosec // test fixture, not real credentials
			// splitScheme rejects a digit-leading prefix (client.go:133), so
			// no scheme is stripped. The leftover "://" makes the first '/'
			// fall inside "1abc:/", so the authority boundary is "1abc:" (no
			// '@') and the ref is returned unchanged. This pins the rejection
			// branch and documents that a malformed "scheme" with embedded
			// userinfo is left intact rather than redacted.
			name: "digit-leading prefix is not a scheme, ref unchanged",
			ref:  "1abc://user:secret@host/path",
			want: "1abc://user:secret@host/path",
		},
		{ //nolint:gosec // test fixture, not real credentials
			// splitScheme rejects an illegal scheme character (client.go:137);
			// same fall-through as above pins that branch.
			name: "illegal-char prefix is not a scheme, ref unchanged",
			ref:  "ab*c://user:secret@host/path",
			want: "ab*c://user:secret@host/path",
		},
		{
			name: "unlisted scheme without userinfo unchanged",
			ref:  "ssh://host/path",
			want: "ssh://host/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactCredentials(tt.ref)
			if got != tt.want {
				t.Errorf("RedactCredentials(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}

func TestStripScheme(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{
			name: "http scheme",
			ref:  "http://localhost:5000/repo",
			want: "localhost:5000/repo",
		},
		{
			name: "https scheme",
			ref:  "https://ghcr.io/org/repo",
			want: "ghcr.io/org/repo",
		},
		{
			name: "no scheme",
			ref:  "ghcr.io/org/repo",
			want: "ghcr.io/org/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripScheme(tt.ref)
			if got != tt.want {
				t.Errorf("stripScheme(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}
