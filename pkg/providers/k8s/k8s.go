package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

func (p Provider) correctDockerRegistry(secrets []corev1.Secret) ([]corev1.Secret, error) {
	for i, secret := range secrets {
		if secret.Type != corev1.SecretTypeDockerConfigJson && secret.Type != corev1.SecretTypeDockercfg {
			continue
		}

		var data []byte
		var exists bool
		if secret.Type == corev1.SecretTypeDockerConfigJson {
			data, exists = secret.Data[corev1.DockerConfigJsonKey]
		} else {
			data, exists = secret.Data[corev1.DockerConfigKey]
		}
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

		if secret.Type == corev1.SecretTypeDockerConfigJson {
			secrets[i].Data[corev1.DockerConfigJsonKey] = updatedData
		} else {
			secrets[i].Data[corev1.DockerConfigKey] = updatedData
		}
	}
	return secrets, nil
}

// Example of incoming data for DockerConfigJson:
// {
//     "auths": {
//         "https://index.docker.io/v1/": {"auth": "xxxx=="},
//         "https://docker.io/v1/": {"auth": "xxxx=="}
//     }
// }
// Example of outgoing data for DockerConfig:
// {
//      "https://index.docker.io/v1/": {"auth": "xxxx=="}
// }

func parseDockerConfig(data []byte) (map[string]json.RawMessage, error) {
	var dockerConfig map[string]json.RawMessage

	if err := json.Unmarshal(data, &dockerConfig); err != nil {
		return nil, fmt.Errorf("unmarshalling docker config: %w", err)
	}

	return dockerConfig, nil
}

func updateDockerRegistryAuths(dockerConfig map[string]json.RawMessage) error {
	if authsRaw, exists := dockerConfig["auths"]; exists {
		var authsMap map[string]json.RawMessage
		if err := json.Unmarshal(authsRaw, &authsMap); err != nil {
			return fmt.Errorf("unmarshalling 'auths': %w", err)
		}

		for url, creds := range authsMap {
			if strings.Contains(url, "docker.io") && url != "https://index.docker.io/v1/" {
				authsMap["https://index.docker.io/v1/"] = creds
				delete(authsMap, url)
			}
		}

		updatedAuthsRaw, err := json.Marshal(authsMap)
		if err != nil {
			return fmt.Errorf("marshalling updated 'auths': %w", err)
		}
		dockerConfig["auths"] = updatedAuthsRaw
	} else {
		for url, creds := range dockerConfig {
			if strings.Contains(url, "docker.io") && url != "https://index.docker.io/v1/" {
				dockerConfig["https://index.docker.io/v1/"] = creds
				delete(dockerConfig, url)
				break
			}
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
