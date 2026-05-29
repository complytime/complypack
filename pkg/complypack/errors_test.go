// SPDX-License-Identifier: Apache-2.0

package complypack_test

import (
	"errors"
	"testing"

	"github.com/complytime/complypack/pkg/complypack"
)

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"invalid config", complypack.ErrInvalidConfig},
		{"empty content", complypack.ErrEmptyContent},
		{"verification failed", complypack.ErrVerificationFailed},
		{"signing failed", complypack.ErrSigningFailed},
		{"invalid media type", complypack.ErrInvalidMediaType},
		{"no content layer", complypack.ErrNoContentLayer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Error("sentinel error is nil")
			}
			if tt.err.Error() == "" {
				t.Error("sentinel error has empty message")
			}
		})
	}
}

func TestErrorWrapping(t *testing.T) {
	// Verify sentinel errors can be used with errors.Is
	wrapped := errors.New("wrapped: " + complypack.ErrInvalidConfig.Error())
	if errors.Is(wrapped, complypack.ErrInvalidConfig) {
		t.Error("string wrapping should not match errors.Is")
	}

	// Proper wrapping
	properWrapped := errors.Join(complypack.ErrInvalidConfig, errors.New("details"))
	if !errors.Is(properWrapped, complypack.ErrInvalidConfig) {
		t.Error("errors.Join should preserve Is matching")
	}
}
