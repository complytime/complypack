// SPDX-License-Identifier: Apache-2.0

package complypack

import (
	"context"
	"errors"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/memory"
)

func TestSigningOptionsValidation(t *testing.T) {
	t.Run("both keyed and keyless signing", func(t *testing.T) {
		opts := &packOptions{
			signingKeyPath:  "/key",
			keylessIdentity: "user@example.com",
			keylessIssuer:   "https://issuer.example.com",
		}

		err := validateSigningOptions(opts)
		if err == nil {
			t.Error("expected error for both signing options set")
		}
		if !errors.Is(err, ErrSigningFailed) {
			t.Errorf("expected ErrSigningFailed, got %v", err)
		}
	})

	t.Run("keyless without issuer", func(t *testing.T) {
		opts := &packOptions{
			keylessIdentity: "user@example.com",
		}

		err := validateSigningOptions(opts)
		if err == nil {
			t.Error("expected error for keyless identity without issuer")
		}
	})

	t.Run("keyless without identity", func(t *testing.T) {
		opts := &packOptions{
			keylessIssuer: "https://issuer.example.com",
		}

		err := validateSigningOptions(opts)
		if err == nil {
			t.Error("expected error for keyless issuer without identity")
		}
	})

	t.Run("keyed signing only", func(t *testing.T) {
		opts := &packOptions{
			signingKeyPath: "/key",
		}

		err := validateSigningOptions(opts)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("keyless signing only", func(t *testing.T) {
		opts := &packOptions{
			keylessIdentity: "user@example.com",
			keylessIssuer:   "https://issuer.example.com",
		}

		err := validateSigningOptions(opts)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("no signing", func(t *testing.T) {
		opts := &packOptions{}

		err := validateSigningOptions(opts)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestSignStub(t *testing.T) {
	// This test verifies the stub returns "not implemented" error
	ctx := context.Background()
	store := memory.New()
	desc := ocispec.Descriptor{}
	opts := &packOptions{signingKeyPath: "/key"}

	err := sign(ctx, store, desc, opts)
	if err == nil {
		t.Error("expected not implemented error from stub")
	}
}
