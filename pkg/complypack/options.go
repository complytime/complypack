// SPDX-License-Identifier: Apache-2.0

package complypack

// PackOption configures optional Pack behavior.
type PackOption func(*packOptions)

// packOptions holds internal state for Pack options.
type packOptions struct {
	// Signing options (mutually exclusive)
	signingKeyPath  string
	keylessIdentity string
	keylessIssuer   string

	// Annotations for OCI manifest
	annotations map[string]string
}

// WithSigning enables keyed signing with the private key at keyPath.
// Mutually exclusive with WithKeylessSigning.
func WithSigning(keyPath string) PackOption {
	return func(o *packOptions) {
		o.signingKeyPath = keyPath
	}
}

// WithKeylessSigning enables keyless signing via OIDC identity and issuer.
// Requires OIDC token in environment (e.g., GitHub Actions, GitLab CI).
// Mutually exclusive with WithSigning.
func WithKeylessSigning(identity, issuer string) PackOption {
	return func(o *packOptions) {
		o.keylessIdentity = identity
		o.keylessIssuer = issuer
	}
}

// WithAnnotations adds OCI manifest annotations.
// Common annotations: org.opencontainers.image.created, org.opencontainers.image.authors
func WithAnnotations(annotations map[string]string) PackOption {
	return func(o *packOptions) {
		if o.annotations == nil {
			o.annotations = make(map[string]string)
		}
		for k, v := range annotations {
			o.annotations[k] = v
		}
	}
}

// UnpackOption configures optional Unpack behavior.
type UnpackOption func(*unpackOptions)

// unpackOptions holds internal state for Unpack options.
type unpackOptions struct {
	// Verification options (mutually exclusive)
	verifyKeyPath  string
	verifyCertPath string
	verifyIssuer   string
	verifyIdentity string
}

// WithVerification enables keyed signature verification with the public key at keyPath.
// Mutually exclusive with WithKeylessVerification.
func WithVerification(keyPath string) UnpackOption {
	return func(o *unpackOptions) {
		o.verifyKeyPath = keyPath
	}
}

// WithKeylessVerification enables keyless signature verification.
// Verifies certificate chain, Rekor inclusion, and OIDC claims.
// Mutually exclusive with WithVerification.
func WithKeylessVerification(certPath, issuer, identity string) UnpackOption {
	return func(o *unpackOptions) {
		o.verifyCertPath = certPath
		o.verifyIssuer = issuer
		o.verifyIdentity = identity
	}
}
