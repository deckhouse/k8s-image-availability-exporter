package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/flant/k8s-image-availability-exporter/pkg/providers/amazon"
	"github.com/google/go-containerregistry/pkg/authn"
)

type Provider interface {
	GetAuthKeychain(ctx context.Context, registryStr string) (authn.Keychain, error)
}

func GetProvider(registryStr string) (Provider, error) {
	switch {
	case strings.Contains(registryStr, "amazonaws.com"):
		return amazon.NewECRProvider(), nil
	default:
		return nil, fmt.Errorf("unsupported registry")
	}
}
