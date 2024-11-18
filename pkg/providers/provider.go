package providers

import (
	corev1 "k8s.io/api/core/v1"
	"regexp"

	"github.com/flant/k8s-image-availability-exporter/pkg/providers/amazon"
	"github.com/flant/k8s-image-availability-exporter/pkg/providers/k8s"
	"github.com/google/go-containerregistry/pkg/authn"
)

type Provider interface {
	GetAuthKeychain(registry string) (authn.Keychain, error)
}

type ProviderRegistry map[string]Provider

func NewProviderChain(pullSecretsGetter func(image string) []corev1.Secret) ProviderRegistry {
	amazonProvider := amazon.NewProvider()
	k8sProvider := k8s.NewProvider(pullSecretsGetter)

	return map[string]Provider{
		"amazon": amazonProvider,
		"k8s":    k8sProvider,
	}
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
