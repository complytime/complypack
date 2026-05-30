// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// NewCredentialFunc returns an auth.CredentialFunc backed by the Docker
// credential resolution chain (credHelpers -> credsStore -> auths).
func NewCredentialFunc() (auth.CredentialFunc, error) {
	store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return nil, err
	}
	return credentials.Credential(store), nil
}
