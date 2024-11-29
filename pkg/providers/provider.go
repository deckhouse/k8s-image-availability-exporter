package providers

import (
	corev1 "k8s.io/api/core/v1"
	"regexp"

	"github.com/google/go-containerregistry/pkg/authn"
)

type Provider interface {
	GetName() string
	GetAuthKeychain(registry string) (authn.Keychain, error)
}

type ProviderRegistry map[string]Provider

func NewProviderChain(providers ...Provider) ProviderRegistry {
	p := make(ProviderRegistry)

	for _, provider := range providers {
		p[provider.GetName()] = provider
	}

	return p
}

type ImagePullSecretsFunc func(image string) []corev1.Secret

var (
	amazonURLRegex = regexp.MustCompile(`^(\d{12})\.dkr\.ecr\.([a-z0-9-]+)\.amazonaws\.com(?:\.cn)?/([^:]+):(.+)$`)
)

func (p ProviderRegistry) GetAuthKeychain(registry string) (authn.Keychain, error) {
	switch {
	case amazonURLRegex.MatchString(registry):
		return p["amazon"].GetAuthKeychain(registry)
	default:
		return p["k8s"].GetAuthKeychain(registry)
	}
}
