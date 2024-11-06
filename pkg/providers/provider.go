package providers

import (
	corev1 "k8s.io/api/core/v1"
	"strings"

	"github.com/flant/k8s-image-availability-exporter/pkg/providers/amazon"
	"github.com/flant/k8s-image-availability-exporter/pkg/providers/k8s"
	"github.com/google/go-containerregistry/pkg/authn"
)

type Provider interface {
	GetAuthKeychain(registryStr string) (authn.Keychain, error)
}

type ProviderRegistry map[string]Provider

func NewProviderChain(pullSecretsGetter func(image string) []corev1.Secret) ProviderRegistry {
	return map[string]Provider{
		"amazon": amazon.NewProvider(),
		"k8s":    k8s.NewProvider(pullSecretsGetter),
	}
}

type ImagePullSecretsFunc func(image string) []corev1.Secret

func (p ProviderRegistry) GetAuthKeychain(registryStr string) (authn.Keychain, error) {
	switch {
	case strings.Contains(registryStr, "amazonaws.com"):
		return p["amazon"].GetAuthKeychain(registryStr)

	default:
		return p["k8s"].GetAuthKeychain(registryStr)
	}
}
