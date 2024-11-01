package providers

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
)

type ContainerRegistryProvider interface {
	GetAuthKeychain(ctx context.Context, registryStr string) (authn.Keychain, error)
}

func FindProviderKeychain(ctx context.Context, registryStr string, providers []ContainerRegistryProvider) (authn.Keychain, error) {
	for _, provider := range providers {
		keychain, err := provider.GetAuthKeychain(ctx, registryStr)
		if err == nil {
			return keychain, nil
		}
	}
	return nil, fmt.Errorf("can't find proper provider for URL: %s", registryStr)
}
