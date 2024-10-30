package providers

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/google/go-containerregistry/pkg/authn"
)

type ECRDetails struct {
	Region string
	Domain string
}

type customKeychain struct {
	authenticator authn.Authenticator
}

func (kc *customKeychain) Resolve(_ authn.Resource) (authn.Authenticator, error) {
	return kc.authenticator, nil
}

func GetECRAuthKeychain(ctx context.Context, registryStr string) (authn.Keychain, error) {
	ecrDetails, err := parseECRDetails(registryStr)
	if err != nil {
		return nil, err
	}

	ecrClient, err := awsRegionalClient(ctx, ecrDetails.Region)
	if err != nil {
		return nil, fmt.Errorf("error loading AWS config: %w", err)
	}

	authTokenOutput, err := ecrClient.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
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

func IsEcrURL(url string) bool {
	parts := strings.SplitN(url, ".", 5)
	if len(parts) <= 3 || !strings.Contains(url, "amazonaws.com") {
		return false
	}
	return true
}

func parseECRDetails(registryStr string) (ECRDetails, error) {
	parts := strings.SplitN(registryStr, ".", 5)
	return ECRDetails{
		Region: parts[3],
		Domain: registryStr,
	}, nil
}

func awsRegionalClient(ctx context.Context, region string) (*ecr.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	client := ecr.NewFromConfig(cfg)
	return client, nil
}
