package k8s

import (
	"context"
	"fmt"
	kubeauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	corev1 "k8s.io/api/core/v1"

	"github.com/google/go-containerregistry/pkg/authn"
)

type Provider struct {
	pullSecretsGetter func(image string) []corev1.Secret
	name              string
}

func NewProvider(pullSecretsGetter func(image string) []corev1.Secret) *Provider {
	return &Provider{
		pullSecretsGetter: pullSecretsGetter,
		name:              "k8s",
	}
}

func (p Provider) GetAuthKeychain(registry string) (authn.Keychain, error) {
	dereferencedPullSecrets := p.pullSecretsGetter(registry)
	kc, err := kubeauth.NewFromPullSecrets(context.TODO(), dereferencedPullSecrets)
	if err != nil {
		return nil, fmt.Errorf("error while processing keychain from secrets: %w", err)
	}
	return kc, nil
}
func (p Provider) GetName() string {
	return p.name
}
