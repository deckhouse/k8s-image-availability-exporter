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

        var dockerConfig map[string]json.RawMessage
        var err error

        if data, exists := secret.Data[corev1.DockerConfigJsonKey]; exists {
            err = json.Unmarshal(data, &dockerConfig)
        } else if data, exists := secret.Data[corev1.DockerConfigKey]; exists {
            // .dockercfg is in a different format, so we need to parse it differently
            dockerConfig, err = parseDockercfg(data)
        } else {
            continue
        }

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

func parseDockercfg(data []byte) (map[string]json.RawMessage, error) {
    var dockercfg struct {
        Auths map[string]json.RawMessage `json:"auths"`
    }
    err := json.Unmarshal(data, &dockercfg)
    if err != nil {
        return nil, err
    }
    return dockercfg.Auths, nil
}

func updateDockerRegistryAuths(dockerConfig map[string]json.RawMessage) error {
    auths, ok := dockerConfig["auths"]
    if !ok {
        return nil
    }

    var authsMap map[string]json.RawMessage
    err := json.Unmarshal(auths, &authsMap)
    if err != nil {
        return err
    }

    for url, creds := range authsMap {
        if strings.Contains(url, "docker.io") && url != "https://index.docker.io/v1/" {
            authsMap["https://index.docker.io/v1/"] = creds
            delete(authsMap, url)
        }
    }

    dockerConfig["auths"], err = json.Marshal(authsMap)
    return err
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
