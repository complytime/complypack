// SPDX-License-Identifier: Apache-2.0

package registry

import "testing"

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
