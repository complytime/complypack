// SPDX-License-Identifier: Apache-2.0

package complypack

import (
	"context"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

// validateSigningOptions checks that signing options are valid.
// Returns ErrSigningFailed if options are misconfigured.
func validateSigningOptions(opts *packOptions) error {
	hasKeyed := opts.signingKeyPath != ""
	hasKeyless := opts.keylessIdentity != "" || opts.keylessIssuer != ""

	// Both signing methods provided
	if hasKeyed && hasKeyless {
		return fmt.Errorf("%w: cannot use both keyed and keyless signing", ErrSigningFailed)
	}

	// Keyless requires both identity and issuer
	if opts.keylessIdentity != "" && opts.keylessIssuer == "" {
		return fmt.Errorf("%w: keyless signing requires both identity and issuer", ErrSigningFailed)
	}
	if opts.keylessIssuer != "" && opts.keylessIdentity == "" {
		return fmt.Errorf("%w: keyless signing requires both identity and issuer", ErrSigningFailed)
	}

	return nil
}

// sign attaches a signature to the OCI artifact using sigstore.
// TODO: Implement actual sigstore-go integration.
func sign(_ context.Context, _ content.Storage, _ ocispec.Descriptor, opts *packOptions) error {
	if err := validateSigningOptions(opts); err != nil {
		return err
	}

	// No signing requested
	if opts.signingKeyPath == "" && opts.keylessIdentity == "" {
		return nil
	}

	// TODO: Implement keyed signing with sigstore-go
	if opts.signingKeyPath != "" {
		return fmt.Errorf("%w: keyed signing not yet implemented", ErrSigningFailed)
	}

	// TODO: Implement keyless signing with sigstore-go
	if opts.keylessIdentity != "" {
		return fmt.Errorf("%w: keyless signing not yet implemented", ErrSigningFailed)
	}

	return nil
}
