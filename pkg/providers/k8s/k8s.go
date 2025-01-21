package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	kubeauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"strings"

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

func (p Provider) correctDockerRegistry(secrets []corev1.Secret) ([]corev1.Secret, error) {
	for i, secret := range secrets {
		if secret.Type != corev1.SecretTypeDockerConfigJson {
			continue
		}

		data, exists := secret.Data[corev1.DockerConfigJsonKey]
		if !exists {
			continue
		}

		dockerConfig, err := parseDockerConfig(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse docker config for secret %d: %w", i, err)
		}

		if err := updateDockerRegistryAuths(dockerConfig); err != nil {
			return nil, fmt.Errorf("failed to update docker registry auths for secret %d: %w", i, err)
		}

		updatedData, err := json.Marshal(dockerConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal updated docker config for secret %d: %w", i, err)
		}

		secrets[i].Data[corev1.DockerConfigJsonKey] = updatedData
	}
	return secrets, nil
}

func parseDockerConfig(data []byte) (map[string]interface{}, error) {
	var dockerConfig map[string]interface{}
	if err := json.Unmarshal(data, &dockerConfig); err != nil {
		return nil, fmt.Errorf("unmarshalling docker config: %w", err)
	}
	return dockerConfig, nil
}

func updateDockerRegistryAuths(dockerConfig map[string]interface{}) error {
	auths, ok := dockerConfig["auths"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("'auths' section is missing or invalid in docker config")
	}

	for url, creds := range auths {
		if strings.Contains(url, "docker.io") && url != "https://index.docker.io/v1/" {
			auths["https://index.docker.io/v1/"] = creds
			delete(auths, url)
		}
	}
	return nil
}


func (p Provider) GetAuthKeychain(registry string) (authn.Keychain, error) {
	dereferencedPullSecrets := p.pullSecretsGetter(registry)
	correctedSecrets, err := p.correctDockerRegistry(dereferencedPullSecrets)
	if err != nil {
		return nil, err
	}
	kc, err := kubeauth.NewFromPullSecrets(context.TODO(), correctedSecrets)
	if err != nil {
		return nil, fmt.Errorf("error while processing keychain from secrets: %w", err)
	}
	return kc, nil
}
func (p Provider) GetName() string {
	return p.name
}
