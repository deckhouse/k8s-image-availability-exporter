package k8s

import (
	"context"
	kubeauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"

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

func (p Provider) GetAuthKeychain(registryStr string) authn.Keychain {
	dereferencedPullSecrets := p.pullSecretsGetter(registryStr)
	kc, err := kubeauth.NewFromPullSecrets(context.TODO(), dereferencedPullSecrets)
	if err != nil {
		logrus.Panic(err)
	}
	return kc
}
