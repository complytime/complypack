// SPDX-License-Identifier: Apache-2.0

package complypack

import (
	"context"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

// validateVerificationOptions checks that verification options are valid.
// Returns ErrVerificationFailed if options are misconfigured.
func validateVerificationOptions(opts *unpackOptions) error {
	hasKeyed := opts.verifyKeyPath != ""
	hasKeyless := opts.verifyCertPath != "" || opts.verifyIssuer != "" || opts.verifyIdentity != ""

	// Both verification methods provided
	if hasKeyed && hasKeyless {
		return fmt.Errorf("%w: cannot use both keyed and keyless verification", ErrVerificationFailed)
	}

	// Keyless requires cert, issuer, and identity
	if hasKeyless {
		if opts.verifyCertPath == "" {
			return fmt.Errorf("%w: keyless verification requires cert path", ErrVerificationFailed)
		}
		if opts.verifyIssuer == "" {
			return fmt.Errorf("%w: keyless verification requires issuer", ErrVerificationFailed)
		}
		if opts.verifyIdentity == "" {
			return fmt.Errorf("%w: keyless verification requires identity", ErrVerificationFailed)
		}
	}

	return nil
}

// verify checks the signature on the OCI artifact using sigstore.
// TODO: Implement actual sigstore-go integration.
func verify(_ context.Context, _ content.ReadOnlyStorage, _ ocispec.Descriptor, opts *unpackOptions) error {
	if err := validateVerificationOptions(opts); err != nil {
		return err
	}

	// No verification requested
	if opts.verifyKeyPath == "" && opts.verifyCertPath == "" {
		return nil
	}

	// TODO: Implement keyed verification with sigstore-go
	if opts.verifyKeyPath != "" {
		return fmt.Errorf("%w: keyed verification not yet implemented", ErrVerificationFailed)
	}

	// TODO: Implement keyless verification with sigstore-go
	if opts.verifyCertPath != "" {
		return fmt.Errorf("%w: keyless verification not yet implemented", ErrVerificationFailed)
	}

	return nil
}
