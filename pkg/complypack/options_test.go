// SPDX-License-Identifier: Apache-2.0

package complypack

import (
	"testing"
)

func TestPackOptions(t *testing.T) {
	t.Run("WithSigning", func(t *testing.T) {
		opts := &packOptions{}
		opt := WithSigning("/path/to/key")
		opt(opts)

		if opts.signingKeyPath != "/path/to/key" {
			t.Errorf("signingKeyPath = %q, want %q", opts.signingKeyPath, "/path/to/key")
		}
	})

	t.Run("WithKeylessSigning", func(t *testing.T) {
		opts := &packOptions{}
		opt := WithKeylessSigning("user@example.com", "https://token.actions.githubusercontent.com")
		opt(opts)

		if opts.keylessIdentity != "user@example.com" {
			t.Errorf("keylessIdentity = %q, want %q", opts.keylessIdentity, "user@example.com")
		}
		if opts.keylessIssuer != "https://token.actions.githubusercontent.com" {
			t.Errorf("keylessIssuer = %q, want %q", opts.keylessIssuer, "https://token.actions.githubusercontent.com")
		}
	})

	t.Run("WithAnnotations", func(t *testing.T) {
		opts := &packOptions{}
		annotations := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}
		opt := WithAnnotations(annotations)
		opt(opts)

		if len(opts.annotations) != 2 {
			t.Errorf("annotations count = %d, want 2", len(opts.annotations))
		}
		if opts.annotations["key1"] != "value1" {
			t.Errorf("annotations[key1] = %q, want %q", opts.annotations["key1"], "value1")
		}
	})

	t.Run("multiple options compose", func(t *testing.T) {
		opts := &packOptions{}
		WithSigning("/key")(opts)
		WithAnnotations(map[string]string{"foo": "bar"})(opts)

		if opts.signingKeyPath != "/key" {
			t.Error("signing option not applied")
		}
		if opts.annotations["foo"] != "bar" {
			t.Error("annotations option not applied")
		}
	})
}

func TestUnpackOptions(t *testing.T) {
	t.Run("WithVerification", func(t *testing.T) {
		opts := &unpackOptions{}
		opt := WithVerification("/path/to/pubkey")
		opt(opts)

		if opts.verifyKeyPath != "/path/to/pubkey" {
			t.Errorf("verifyKeyPath = %q, want %q", opts.verifyKeyPath, "/path/to/pubkey")
		}
	})

	t.Run("WithKeylessVerification", func(t *testing.T) {
		opts := &unpackOptions{}
		opt := WithKeylessVerification("/cert", "https://issuer.example.com", "user@example.com")
		opt(opts)

		if opts.verifyCertPath != "/cert" {
			t.Errorf("verifyCertPath = %q, want %q", opts.verifyCertPath, "/cert")
		}
		if opts.verifyIssuer != "https://issuer.example.com" {
			t.Errorf("verifyIssuer = %q, want %q", opts.verifyIssuer, "https://issuer.example.com")
		}
		if opts.verifyIdentity != "user@example.com" {
			t.Errorf("verifyIdentity = %q, want %q", opts.verifyIdentity, "user@example.com")
		}
	})
}
