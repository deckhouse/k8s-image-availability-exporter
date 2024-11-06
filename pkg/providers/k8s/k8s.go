package k8s

import (
	"context"
	kubeauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
)

type Provider struct {
	pullSecretsGetter func(image string) []corev1.Secret
}

func NewProvider(pullSecretsGetter func(image string) []corev1.Secret) *Provider {
	return &Provider{
		pullSecretsGetter: pullSecretsGetter,
	}
}

func (p Provider) GetAuthKeychain(registryStr string) (authn.Keychain, error) {
	dereferencedPullSecrets := p.pullSecretsGetter(registryStr)
	kc, err := kubeauth.NewFromPullSecrets(context.TODO(), dereferencedPullSecrets)
	if err != nil {
		return nil, fmt.Errorf("error while processing keychain from secrets: %w", err)
	}
	return kc, nil
}
