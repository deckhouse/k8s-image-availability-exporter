package amazon

import (
	"context"
	"encoding/base64"
	"strings"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/google/go-containerregistry/pkg/authn"
)

type Provider struct{}

func NewProvider() *Provider {
	return &Provider{}
}

func (p Provider) GetAuthKeychain(registryStr string) (authn.Keychain, error) {
	ecrClient, err := awsRegionalClient(context.TODO(), parseECRDetails(registryStr))
	if err != nil {
		return nil, fmt.Errorf("error loading AWS config: %w", err)
	}

	authTokenOutput, err := ecrClient.GetAuthorizationToken(context.TODO(), &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return nil, fmt.Errorf("error getting ECR authorization token: %w", err)
	}


	if len(authTokenOutput.AuthorizationData) == 0 {
		return nil, fmt.Errorf("no authorization data received from ECR")
	}

	authData := authTokenOutput.AuthorizationData[0]
	decodedToken, err := base64.StdEncoding.DecodeString(*authData.AuthorizationToken)
	if err != nil {
		return nil, fmt.Errorf("error decoding authorization token: %w", err)
	}

	credentials := strings.SplitN(string(decodedToken), ":", 2)
	if len(credentials) != 2 {
		return nil, fmt.Errorf("invalid authorization token format")
	}
	authConfig := authn.AuthConfig{
		Username: credentials[0],
		Password: credentials[1],
	}
	auth := authn.FromConfig(authConfig)
	return &customKeychain{authenticator: auth}, nil
}

func parseECRDetails(registryStr string) string {
	parts := strings.SplitN(registryStr, ".", 5)
	if len(parts) < 3 {
		return ""
	}
	return parts[3]
}

func awsRegionalClient(ctx context.Context, region string) (*ecr.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	client := ecr.NewFromConfig(cfg)
	return client, nil
}

type customKeychain struct {
	authenticator authn.Authenticator
}

func (kc *customKeychain) Resolve(_ authn.Resource) (authn.Authenticator, error) {
	return kc.authenticator, nil
}
